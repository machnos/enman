package influx

import (
	"context"
	"crypto/tls"
	"embed"
	"enman/internal/log"
	"enman/internal/persistency"
	"fmt"
	"github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/domain"
	"net"
	"net/http"
	"strings"
	"text/template"
	"time"
)

const (
	organization          = "machnos-enman"
	bucketEnergyFlowHour  = bucketEnergyFlow + "_hour"
	bucketEnergyFlowDay   = bucketEnergyFlow + "_day"
	bucketEnergyFlowMonth = bucketEnergyFlow + "_month"
	queryTimeout          = 30
)

//go:embed influx_task_energy-flow_aggregation.tmpl
var templateContent embed.FS

type influxRepository struct {
	persistency.Repository
	client    influxdb2.Client
	writeApis map[string]api.WriteAPI
	queryApi  api.QueryAPI
}

func NewInfluxRepository(serverUrl string, token string) persistency.Repository {
	httpClient := &http.Client{
		Timeout: time.Second * time.Duration(queryTimeout),
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout: 5 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout: 5 * time.Second,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
			IdleConnTimeout:     90 * time.Second,
		},
	}
	repo := &influxRepository{
		client:    influxdb2.NewClientWithOptions(serverUrl, token, influxdb2.DefaultOptions().SetHTTPClient(httpClient)),
		writeApis: make(map[string]api.WriteAPI),
	}
	repo.writeApis[bucketEnergyFlow] = repo.client.WriteAPI(organization, bucketEnergyFlow)
	repo.writeApis[bucketPrices] = repo.client.WriteAPI(organization, bucketPrices)
	repo.queryApi = repo.client.QueryAPI(organization)
	return repo
}

func (i *influxRepository) Initialize() error {
	org, err := i.client.OrganizationsAPI().FindOrganizationByName(context.Background(), organization)
	if org == nil {
		desc := "The Machnos EnMan organization manages your energy needs."
		org, err = i.client.OrganizationsAPI().CreateOrganization(context.Background(), &domain.Organization{
			Name:        organization,
			Description: &desc,
		})
		if err != nil {
			return err
		}
	}
	if err != nil {
		return err
	}
	err = i.createBucketWhenNotPresent(org, bucketEnergyFlow, int64((time.Hour * 24 * 90).Seconds()))
	if err != nil {
		return err
	}
	err = i.createBucketWhenNotPresent(org, bucketEnergyFlowHour, 0)
	if err != nil {
		return err
	}
	err = i.createBucketWhenNotPresent(org, bucketEnergyFlowDay, 0)
	if err != nil {
		return err
	}
	err = i.createBucketWhenNotPresent(org, bucketEnergyFlowMonth, 0)
	if err != nil {
		return err
	}
	err = i.createBucketWhenNotPresent(org, bucketPrices, 0)
	if err != nil {
		return err
	}
	err = i.createTaskWhenNotPresent(org,
		"Energy flow per hour",
		"1h",
		"influx_task_energy-flow_aggregation.tmpl",
		map[string]string{
			"SourceBucket": bucketEnergyFlow,
			"TargetBucket": bucketEnergyFlowHour,
			"Measurement":  measurementEnergyUsage,
		},
	)
	if err != nil {
		return err
	}
	err = i.createTaskWhenNotPresent(org,
		"Energy flow per day",
		"1d",
		"influx_task_energy-flow_aggregation.tmpl",
		map[string]string{
			"SourceBucket": bucketEnergyFlow,
			"TargetBucket": bucketEnergyFlowDay,
			"Measurement":  measurementEnergyUsage,
		},
	)
	if err != nil {
		return err
	}
	err = i.createTaskWhenNotPresent(org,
		"Energy flow per month",
		"1mo",
		"influx_task_energy-flow_aggregation.tmpl",
		map[string]string{
			"SourceBucket": bucketEnergyFlow,
			"TargetBucket": bucketEnergyFlowMonth,
			"Measurement":  measurementEnergyUsage,
		},
	)
	if err != nil {
		return err
	}
	return nil
}

func (i *influxRepository) createBucketWhenNotPresent(organization *domain.Organization, bucketName string, expiration int64) error {
	bucketApi := i.client.BucketsAPI()
	_, err := bucketApi.FindBucketByName(context.Background(), bucketName)
	if err != nil {
		retentionType := domain.RetentionRuleTypeExpire
		_, err := bucketApi.CreateBucketWithName(context.Background(), organization, bucketName, domain.RetentionRule{
			Type:         &retentionType,
			EverySeconds: expiration,
		})
		if err != nil {
			return err
		}
		if log.InfoEnabled() {
			log.Infof("Created bucket '%s' in organization '%s'", bucketName, organization.Name)
		}
	}
	return nil
}

func (i *influxRepository) createTaskWhenNotPresent(organization *domain.Organization, taskName string, taskInterval, templateLocation string, templateData any) error {
	taskApi := i.client.TasksAPI()
	tasks, err := taskApi.FindTasks(context.Background(), &api.TaskFilter{Name: taskName})
	if err != nil {
		return err
	}
	if len(tasks) == 0 {
		taskTemplate, err := template.ParseFS(templateContent, templateLocation)
		if err != nil {
			return err
		}
		res := new(strings.Builder)
		err = taskTemplate.Execute(res, templateData)
		if err != nil {
			return err
		}
		_, err = taskApi.CreateTaskWithEvery(context.Background(), taskName, res.String(), taskInterval, *organization.Id)
		if err != nil {
			return err
		}
		if log.InfoEnabled() {
			log.Infof("Created task '%s' in organization '%s'", taskName, organization.Name)
		}
	}
	return nil
}

func (i *influxRepository) Close() {
	for _, value := range i.writeApis {
		value.Flush()
	}
	i.client.Close()
}

func (i *influxRepository) toInfluxDuration(unit persistency.WindowUnit, amount uint16) string {
	switch unit {
	case persistency.Nanosecond:
		return fmt.Sprintf("%dns", amount)
	case persistency.Microsecond:
		return fmt.Sprintf("%dus", amount)
	case persistency.Millisecond:
		return fmt.Sprintf("%dms", amount)
	case persistency.Second:
		return fmt.Sprintf("%ds", amount)
	case persistency.Minute:
		return fmt.Sprintf("%dm", amount)
	case persistency.Hour:
		return fmt.Sprintf("%dh", amount)
	case persistency.Day:
		return fmt.Sprintf("%dd", amount)
	case persistency.Week:
		return fmt.Sprintf("%dw", amount)
	case persistency.Month:
		return fmt.Sprintf("%dmo", amount)
	case persistency.Year:
		return fmt.Sprintf("%dy", amount)
	}
	return ""
}

func (i *influxRepository) toInfluxFunction(function persistency.AggregateFunction) string {
	_, ok := function.(persistency.Count)
	if ok {
		return "count"
	}
	_, ok = function.(persistency.Max)
	if ok {
		return "max"
	}
	_, ok = function.(persistency.Mean)
	if ok {
		return "mean"
	}
	_, ok = function.(persistency.Median)
	if ok {
		return "median"
	}
	_, ok = function.(persistency.Min)
	if ok {
		return "min"
	}
	return ""
}

func (i *influxRepository) toAggregateWindow(aggregateConfiguration *persistency.AggregateConfiguration) string {
	aggregateWindow := fmt.Sprintf("every: %s, fn: %s, createEmpty: %t",
		i.toInfluxDuration(aggregateConfiguration.WindowUnit, aggregateConfiguration.WindowAmount),
		i.toInfluxFunction(aggregateConfiguration.Function),
		aggregateConfiguration.CreateEmpty,
	)
	return aggregateWindow
}
