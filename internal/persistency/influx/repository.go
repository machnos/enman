package influx

import (
	"context"
	"crypto/tls"
	"embed"
	enmandomain "enman/internal/domain"
	"enman/internal/log"
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
	organization                   = "machnos-enman"
	bucketElectricityHour          = bucketElectricity + "_hour"
	bucketElectricityDay           = bucketElectricity + "_day"
	bucketElectricityMonth         = bucketElectricity + "_month"
	bucketGasHour                  = bucketGas + "_hour"
	bucketGasDay                   = bucketGas + "_day"
	bucketGasMonth                 = bucketGas + "_month"
	bucketWaterHour                = bucketWater + "_hour"
	bucketWaterDay                 = bucketWater + "_day"
	bucketWaterMonth               = bucketWater + "_month"
	bucketBatteryHour              = bucketBattery + "_hour"
	bucketBatteryDay               = bucketBattery + "_day"
	bucketBatteryMonth             = bucketBattery + "_month"
	queryTimeout                   = 30
	electricityAggregationTemplate = "influx_task_electricity_aggregation.tmpl"
	measurementUsage               = "usage"
	tagName                        = "name"
	tagRole                        = "role"
)

//go:embed influx_task_electricity_aggregation.tmpl
var templateContent embed.FS

type influxRepository struct {
	enmandomain.Repository
	client    influxdb2.Client
	writeApis map[string]api.WriteAPI
	queryApi  api.QueryAPI
}

func NewInfluxRepository(serverUrl string, token string) enmandomain.Repository {
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
	repo.writeApis[bucketElectricity] = repo.client.WriteAPI(organization, bucketElectricity)
	repo.writeApis[bucketGas] = repo.client.WriteAPI(organization, bucketGas)
	repo.writeApis[bucketWater] = repo.client.WriteAPI(organization, bucketWater)
	repo.writeApis[bucketBattery] = repo.client.WriteAPI(organization, bucketBattery)
	repo.writeApis[bucketElectricity] = repo.client.WriteAPI(organization, bucketElectricity)
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
	err = i.createBucketWhenNotPresent(org, bucketElectricity, int64((time.Hour * 24 * 90).Seconds()))
	if err != nil {
		return err
	}
	err = i.createBucketWhenNotPresent(org, bucketElectricityHour, 0)
	if err != nil {
		return err
	}
	err = i.createBucketWhenNotPresent(org, bucketElectricityDay, 0)
	if err != nil {
		return err
	}
	err = i.createBucketWhenNotPresent(org, bucketElectricityMonth, 0)
	if err != nil {
		return err
	}
	err = i.createBucketWhenNotPresent(org, bucketPrices, 0)
	if err != nil {
		return err
	}
	err = i.createBucketWhenNotPresent(org, bucketGas, int64((time.Hour * 24 * 90).Seconds()))
	if err != nil {
		return err
	}
	err = i.createBucketWhenNotPresent(org, bucketGasHour, 0)
	if err != nil {
		return err
	}
	err = i.createBucketWhenNotPresent(org, bucketGasDay, 0)
	if err != nil {
		return err
	}
	err = i.createBucketWhenNotPresent(org, bucketGasMonth, 0)
	if err != nil {
		return err
	}
	err = i.createBucketWhenNotPresent(org, bucketWater, int64((time.Hour * 24 * 90).Seconds()))
	if err != nil {
		return err
	}
	err = i.createBucketWhenNotPresent(org, bucketWaterHour, 0)
	if err != nil {
		return err
	}
	err = i.createBucketWhenNotPresent(org, bucketWaterDay, 0)
	if err != nil {
		return err
	}
	err = i.createBucketWhenNotPresent(org, bucketWaterMonth, 0)
	if err != nil {
		return err
	}
	err = i.createBucketWhenNotPresent(org, bucketBattery, int64((time.Hour * 24 * 90).Seconds()))
	if err != nil {
		return err
	}
	err = i.createBucketWhenNotPresent(org, bucketBatteryHour, 0)
	if err != nil {
		return err
	}
	err = i.createBucketWhenNotPresent(org, bucketBatteryDay, 0)
	if err != nil {
		return err
	}
	err = i.createBucketWhenNotPresent(org, bucketBatteryMonth, 0)
	if err != nil {
		return err
	}
	// TODO migrate tasks logic to application (also for gas, water & battery)
	err = i.createTaskWhenNotPresent(org,
		"Electricity per hour",
		"1h",
		electricityAggregationTemplate,
		map[string]string{
			"SourceBucket": bucketElectricity,
			"TargetBucket": bucketElectricityHour,
			"Measurement":  measurementUsage,
		},
	)
	if err != nil {
		return err
	}
	err = i.createTaskWhenNotPresent(org,
		"Electricity per day",
		"1d",
		electricityAggregationTemplate,
		map[string]string{
			"SourceBucket": bucketElectricity,
			"TargetBucket": bucketElectricityDay,
			"Measurement":  measurementUsage,
		},
	)
	if err != nil {
		return err
	}
	err = i.createTaskWhenNotPresent(org,
		"Electricity per month",
		"1mo",
		electricityAggregationTemplate,
		map[string]string{
			"SourceBucket": bucketElectricity,
			"TargetBucket": bucketElectricityMonth,
			"Measurement":  measurementUsage,
		},
	)
	if err != nil {
		return err
	}
	enmandomain.ElectricityMeterReadings.Register(&ElectricityMeterValueChangeListener{repo: i}, nil)
	enmandomain.ElectricityCosts.Register(&ElectricityCostsValueChangeListener{repo: i}, nil)
	enmandomain.GasMeterReadings.Register(&GasMeterValueChangeListener{repo: i}, nil)
	enmandomain.WaterMeterReadings.Register(&WaterMeterValueChangeListener{repo: i}, nil)
	enmandomain.BatteryMeterReadings.Register(&BatteryMeterValueChangeListener{repo: i}, nil)
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
	enmandomain.ElectricityMeterReadings.Deregister(&ElectricityMeterValueChangeListener{repo: i})
	enmandomain.ElectricityCosts.Deregister(&ElectricityCostsValueChangeListener{repo: i})
	for _, value := range i.writeApis {
		value.Flush()
	}
	i.client.Close()
}

func (i *influxRepository) toInfluxDuration(unit enmandomain.WindowUnit, amount uint64) string {
	switch unit {
	case enmandomain.WindowUnitNanosecond:
		return fmt.Sprintf("%dns", amount)
	case enmandomain.WindowUnitMicrosecond:
		return fmt.Sprintf("%dus", amount)
	case enmandomain.WindowUnitMillisecond:
		return fmt.Sprintf("%dms", amount)
	case enmandomain.WindowUnitSecond:
		return fmt.Sprintf("%ds", amount)
	case enmandomain.WindowUnitMinute:
		return fmt.Sprintf("%dm", amount)
	case enmandomain.WindowUnitHour:
		return fmt.Sprintf("%dh", amount)
	case enmandomain.WindowUnitDay:
		return fmt.Sprintf("%dd", amount)
	case enmandomain.WindowUnitWeek:
		return fmt.Sprintf("%dw", amount)
	case enmandomain.WindowUnitMonth:
		return fmt.Sprintf("%dmo", amount)
	case enmandomain.WindowUnitYear:
		return fmt.Sprintf("%dy", amount)
	}
	return ""
}

func (i *influxRepository) toInfluxFunction(function enmandomain.AggregateFunction) string {
	switch function.(type) {
	case enmandomain.Count:
		return "count"
	case enmandomain.Max:
		return "max"
	case enmandomain.Mean:
		return "mean"
	case enmandomain.Median:
		return "median"
	case enmandomain.Min:
		return "min"
	default:
		return ""
	}
}

func (i *influxRepository) toInfluxAggregateFunction(function enmandomain.AggregateFunction) AggregateFunction {
	switch function.(type) {
	case enmandomain.Count:
		return Count
	case enmandomain.Max:
		return Max
	case enmandomain.Mean:
		return Mean
	case enmandomain.Median:
		return Median
	case enmandomain.Min:
		return Min
	case enmandomain.Sum:
		return Sum
	default:
		return ""
	}
}

func (i *influxRepository) toAggregateWindow(aggregateConfiguration *enmandomain.AggregateConfiguration) string {
	aggregateWindow := fmt.Sprintf("every: %s, fn: %s, createEmpty: %t",
		i.toInfluxDuration(aggregateConfiguration.WindowUnit, aggregateConfiguration.WindowAmount),
		i.toInfluxFunction(aggregateConfiguration.Function),
		aggregateConfiguration.CreateEmpty,
	)
	if enmandomain.WindowUnitWeek == aggregateConfiguration.WindowUnit {
		// In InlfuxDB the week starts at wednesday. See https://docs.influxdata.com/flux/v0.x/stdlib/universe/aggregatewindow/#downsample-by-calendar-week-starting-on-monday
		aggregateWindow += ", offset: -3d"
	}
	return aggregateWindow
}

func (i *influxRepository) toAggregateWindowStatement(aggregateConfiguration *enmandomain.AggregateConfiguration) *AggregateWindowStatement {
	statement := NewAggregateWindowStatement(i.toInfluxDuration(aggregateConfiguration.WindowUnit,
		aggregateConfiguration.WindowAmount),
		i.toInfluxAggregateFunction(aggregateConfiguration.Function),
		aggregateConfiguration.CreateEmpty)
	if enmandomain.WindowUnitWeek == aggregateConfiguration.WindowUnit {
		// In InlfuxDB the week starts at wednesday. See https://docs.influxdata.com/flux/v0.x/stdlib/universe/aggregatewindow/#downsample-by-calendar-week-starting-on-monday
		statement.SetOffset("-3d")
	}
	return statement
}
