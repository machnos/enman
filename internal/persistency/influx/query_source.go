package influx

import (
	"fmt"
	"time"
)

type QuerySource interface {
	QuerySourceString() string
}

type BucketQuerySource struct {
	bucket string
}

func (bqs *BucketQuerySource) QuerySourceString() string {
	return fmt.Sprintf(`from(bucket: "%s")`, bqs.bucket)
}

func NewBucketQuerySource(bucket string) *BucketQuerySource {
	return &BucketQuerySource{
		bucket: bucket,
	}
}

type SchemaTagValuesQuerySource struct {
	bucket string
	tag    string
	start  time.Time
	stop   time.Time
}

func (sqs *SchemaTagValuesQuerySource) QuerySourceString() string {
	start := "-30d"
	if !sqs.start.IsZero() {
		start = fmt.Sprintf("%d", sqs.start.Unix())
	}
	stop := "now()"
	if !sqs.stop.IsZero() {
		stop = fmt.Sprintf("%d", sqs.stop.Unix())
	}
	return fmt.Sprintf(`import "influxdata/influxdb/schema" schema.tagValues(bucket: "%s", tag: "%s", start: %s, stop: %s)`, sqs.bucket, sqs.tag, start, stop)
}

func (sqs *SchemaTagValuesQuerySource) SetFrom(from time.Time) *SchemaTagValuesQuerySource {
	sqs.start = from
	return sqs
}

func (sqs *SchemaTagValuesQuerySource) SetTill(till time.Time) *SchemaTagValuesQuerySource {
	sqs.stop = till
	return sqs
}

func NewSchemaTagValuesQuery(bucket string, tag string) *SchemaTagValuesQuerySource {
	return &SchemaTagValuesQuerySource{
		bucket: bucket,
		tag:    tag,
	}
}
