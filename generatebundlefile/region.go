package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cloudwatchtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

var RegionList = []string{
	"us-east-2",
	"us-east-1",
	"us-west-1",
	"us-west-2",
	"ap-northeast-3",
	"ap-northeast-2",
	"ap-southeast-1",
	"ap-southeast-2",
	"ap-northeast-1",
	"ca-central-1",
	"eu-central-1",
	"eu-west-1",
	"eu-west-2",
	"eu-west-3",
	"eu-north-1",
	"sa-east-1",
}

type RepositoryCloudWatch struct {
	Repository string  `json:"repository,omitempty"`
	Digest     string  `json:"digest,omitempty"`
	Region     string  `json:"region,omitempty"`
	TotalHits  int     `json:"totalhits,omitempty"`
	Percent    float64 `json:"percent,omitempty"`
	K8sVersion string  `json:"k8sversion,omitempty"`
}

func FormCloudWatchData(metricData []cloudwatchtypes.MetricDatum, name string, value float64) []cloudwatchtypes.MetricDatum {
	regionName := "region-availibity"
	d := []cloudwatchtypes.Dimension{
		{
			Name:  &regionName,
			Value: &name,
		},
	}
	metricDataPoint := &cloudwatchtypes.MetricDatum{
		Dimensions: d,
		MetricName: &name,
		Value:      &value,
	}
	metricData = append(metricData, *metricDataPoint)
	return metricData
}

func PushCloudWatchRegionCheckData(c *cloudwatch.Client, data []cloudwatchtypes.MetricDatum, k8s_version string) error {
	Input := &cloudwatch.PutMetricDataInput{
		MetricData: data,
		Namespace:  &k8s_version,
	}
	err := pushMetricData(c, *Input)
	if err != nil {
		return err
	}
	return nil
}

func pushMetricData(c *cloudwatch.Client, input cloudwatch.PutMetricDataInput) error {
	_, err := c.PutMetricData(context.TODO(), &input)
	if err != nil {
		return fmt.Errorf("sending cloudwatch PutMetricData(): %w", err)
	}
	return nil
}
