package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

type SlackResponse struct {
	Version string       `json:"version"`
	Source  string       `json:"source"`
	Content SlackContent `json:"content"`
}

type SlackContent struct {
	Description string `json:"description"`
}

func GetParamsFromDynamoDb(pKeyName string, ev envValues) (map[string]string, error) {
	logger.Info("Getting parameters from DynamoDB")

	// Load SDK Config and Create DynamoDB Client
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(ev.dynamoDbRegion))
	if err != nil {
		logger.Error("Unable to load SDK config: " + err.Error())
		return nil, err
	}
	dynamoDbClient := dynamodb.NewFromConfig(cfg)

	resItems, err := dynamoDbClient.GetItem(context.TODO(), &dynamodb.GetItemInput{
		TableName: &ev.dynamoDbTableName,
		Key: map[string]ddtypes.AttributeValue{
			"awsService": &ddtypes.AttributeValueMemberS{Value: pKeyName},
		},
	})
	if err != nil {
		logger.Error("Error getting item from DynamoDB: " + err.Error())
		return nil, err
	}

	returnMap := make(map[string]string)
	for k, v := range resItems.Item {
		if av, ok := v.(*ddtypes.AttributeValueMemberS); ok {
			returnMap[k] = av.Value
		}
	}

	return returnMap, nil
}

func GetSimpleMetricsData(pKeyName string, ev envValues) ([]byte, error) {
	logger.Info("Getting simple metrics data. " + pKeyName)

	// Load SDK Config and Create CloudWatch Client
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(ev.dynamoDbRegion))
	if err != nil {
		logger.Error("Unable to load SDK config: " + err.Error())
		return nil, err
	}
	cwClient := cloudwatch.NewFromConfig(cfg)

	// Get Parameters from DynamoDB
	params, err := GetParamsFromDynamoDb(pKeyName, ev)
	if err != nil {
		return nil, err
	}

	// Get Current Time
	endTime := time.Now()
	dateRangeInt, err := strconv.Atoi(params["dateRange"])
	if err != nil {
		logger.Error("Error converting dateRange to integer: " + err.Error())
		return nil, err
	}
	startTime := endTime.AddDate(0, 0, -dateRangeInt)

	// Set Metric Data Input
	id := "m1"
	namespace := params["namespace"]
	metricName := params["metricName"]
	var dims []map[string]string
	err = json.Unmarshal([]byte(params["dimensions"]), &dims)
	if err != nil {
		logger.Error("Error unmarshalling dimensions: " + err.Error())
		return nil, err
	}
	dimensions := make([]cwtypes.Dimension, 0, len(dims))
	for _, v := range dims {
		for name, value := range v {
			dimensions = append(dimensions, cwtypes.Dimension{
				Name:  &name,
				Value: &value,
			})
		}
	}
	periodStr := params["period"]
	period64, err := strconv.ParseInt(periodStr, 10, 32)
	if err != nil {
		logger.Error("Error converting period to integer: " + err.Error())
		return nil, err
	}
	period := int32(period64)
	stat := params["stat"]
	unit := params["unit"]

	resMetrics, err := cwClient.GetMetricData(context.TODO(), &cloudwatch.GetMetricDataInput{
		StartTime: &startTime,
		EndTime:   &endTime,
		MetricDataQueries: []cwtypes.MetricDataQuery{
			{
				Id: &id,
				MetricStat: &cwtypes.MetricStat{
					Metric: &cwtypes.Metric{
						Namespace:  &namespace,
						MetricName: &metricName,
						Dimensions: dimensions,
					},
					Period: &period,
					Stat:   &stat,
					Unit:   cwtypes.StandardUnit(unit),
				},
			},
		},
	})
	if err != nil {
		logger.Error("Error getting metric data: " + err.Error())
		return nil, err
	}

	// marshal the response
	jst := time.FixedZone("Asia/Tokyo", 9*60*60) // UTC to JST
	csvOutput := make([][]string, len(resMetrics.MetricDataResults[0].Timestamps)+1)
	csvOutput[0] = []string{"timestamp", "value"} // CSV Header
	for i := range resMetrics.MetricDataResults[0].Timestamps {
		csvOutput[i+1] = []string{
			resMetrics.MetricDataResults[0].Timestamps[i].In(jst).Format(time.RFC3339),
			fmt.Sprintf("%v", resMetrics.MetricDataResults[0].Values[i]),
		}
	}

	var b []byte
	buf := bytes.NewBuffer(b)
	writer := csv.NewWriter(buf)
	for _, record := range csvOutput {
		writer.Write(record)
	}
	writer.Flush()
	csvData := buf.Bytes()
	return csvData, nil
}

func SendMessageSns(ev envValues, desc string) error {
	logger.Info("Send message to SNS Topic")

	if ev.snsTopicArn == "" || strings.EqualFold(ev.snsTopicArn, "none") {
		logger.Info("No SNS mode. Skipping.")
		return nil
	}

	// Load SDK Config and Create SNS Client
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(ev.dynamoDbRegion))
	if err != nil {
		logger.Error("Unable to load SDK config: " + err.Error())
		return err
	}
	snsClient := sns.NewFromConfig(cfg)

	var slackMessage SlackResponse = SlackResponse{
		Version: "1.0",
		Source:  "custom",
		Content: SlackContent{
			Description: "*Daily Dashboard Analyzer*\n" + desc,
		},
	}
	slackMessageJson, err := json.Marshal(slackMessage)
	if err != nil {
		logger.Error("Error marshalling test message: " + err.Error())
		return err
	}
	slackMessageStr := string(slackMessageJson)

	// Call SNS Publish
	_, err = snsClient.Publish(context.TODO(), &sns.PublishInput{
		TopicArn: &ev.snsTopicArn,
		Message:  &slackMessageStr,
	})
	if err != nil {
		logger.Error("Error sending message to SNS: " + err.Error())
		return err
	}

	return nil
}
