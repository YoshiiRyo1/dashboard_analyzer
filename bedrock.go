package main

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

func ConverseModel(ev envValues) (string, error) {
	logger.Info("Conversing with the model...")

	// Load SDK Config
	cfgBedrock, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(ev.bedrockRegion))
	if err != nil {
		logger.Error("Unable to load SDK config: " + err.Error())
		return "", err
	}
	bedrockRuntimeClient := bedrockruntime.NewFromConfig(cfgBedrock)

	// Length of metricsName
	messageData := make([][]byte, len(ev.metricsName))

	for i := range ev.metricsName {
		// Get Simple Metrics Data
		res, err := GetSimpleMetricsData(ev.metricsName[i], ev)
		if err != nil {
			logger.Error("Error getting simple metrics data: " + err.Error())
			return "", err
		}
		messageData[i] = res
	}

	// Get System String
	systemStr := getSystemStr()
	system := []types.SystemContentBlock{
		&types.SystemContentBlockMemberText{Value: systemStr},
	}

	// Define the User Message
	var contentBlocks []types.ContentBlock
	for i := range ev.metricsName {
		contentBlocks = append(contentBlocks, &types.ContentBlockMemberText{
			Value: "This document contains " + ev.metricsName[i],
		})
		contentBlocks = append(contentBlocks, &types.ContentBlockMemberDocument{
			Value: types.DocumentBlock{
				Format: types.DocumentFormatCsv,
				Name:   &ev.metricsName[i],
				Source: &types.DocumentSourceMemberBytes{Value: messageData[i]},
			},
		})
	}
	message := types.Message{
		Role:    types.ConversationRoleUser,
		Content: contentBlocks,
	}

	// Define Inference parameters to pass to the model.
	// [Inference parameters for foundation models]: https://docs.aws.amazon.com/bedrock/latest/userguide/model-parameters.html
	var maxToken int32 = 4000
	var temperature float32 = 0.1
	var stopSequence []string = []string{"\n\nHuman:"}
	infenceConfig := types.InferenceConfiguration{
		MaxTokens:     &maxToken,
		Temperature:   &temperature,
		StopSequences: stopSequence,
	}

	// Invoke the model
	modelOutput, err := bedrockRuntimeClient.Converse(context.TODO(), &bedrockruntime.ConverseInput{
		ModelId:         aws.String(ev.bedrockModelId),
		Messages:        []types.Message{message},
		System:          system,
		InferenceConfig: &infenceConfig,
	})
	if err != nil {
		logger.Error("failed to invoke model " + err.Error())
		return "", err
	}

	// Return the response
	var outputStr string
	switch v := modelOutput.Output.(type) {
	case *types.ConverseOutputMemberMessage:
		outputStr = v.Value.Content[0].(*types.ContentBlockMemberText).Value
	case *types.UnknownUnionMember:
		logger.Error("failed to get output")
	default:
		logger.Error("Output is nil or unknown type")
	}

	return outputStr, nil
}

func getSystemStr() string {
	systemStr := `
You are a Senior SRE Engineer working in a team developing a smartphone application.
One of your tasks is to detect and prevent application performance degradation and failures.
You are required to correlate various metric data with timestamps and find correlations between them, and report back to us in terms of the following
Increased number of ALB requests is expected to impact response and latency. Please detect changes in response time and latency per request count and tell us if you determine that the increase in requests has caused a sudden performance degradation.

# Perspective
- See [Explanation of Metrics Data](#explanation-of-metrics-data) for the meaning of metrics data
	- ALB target response time, less than 0.5 seconds is considered normal
	- Aurora DML Latency, less than 1.5 milliseconds is considered normal
	- Aurora Select latency, less than 0.4 milliseconds is considered normal
- Detecting performance degradation and its signs
	- Reports a pattern of slowly worsening metrics
- Detects the occurrence or suspicion of a failure
	- Reports a metric pattern of sudden fluctuations in metric data
	- Report a metric pattern where metric data is suddenly missing
- Report any extreme up/down fluctuations in metrics data during a period of time
	- Ignore if it fluctuates by 10%% in an hour
- Metrics data include data from the past few days, but the primary data to investigate are those within 24 hours of the present
	- Use data from the past few days as a performance baseline

# Explanation of Metrics Data
The types of metrics data to be obtained and their descriptions are shown below.

## ALB Requests
Number of requests processed via IPv4 and IPv6. This metric is only incremented for requests for which the load balancer node was able to select a target. Requests that are rejected before a target is selected are not reflected in this metric.

## ALB Target Response Time
The time (in seconds) elapsed after a request leaves the load balancer before the target begins sending response headers.

## Aurora DML Latency
Average time (in milliseconds) for inserts, updates, and deletes against the database cluster.

## Aurora Select Latency
Average time (in milliseconds) for select queries against the database cluster.

## ECS CPU Utilization
Number of CPU units used by the task for the resource specified by the dimension set being used.

## Reporting format
Please report in markdown format. Please summarize the main points using bullet points and provide supporting data.
Report only objective facts based on data; do not include speculation or suggestions for improvement.
If no abnormality is found, please report “no abnormality”.
Please summarize your report in less than 4000 bytes and describe in Japanese.
`

	return systemStr
}
