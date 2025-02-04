package main

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/lambda"
)

type envValues struct {
	dynamoDbTableName string
	dynamoDbRegion    string
	bedrockRegion     string
	bedrockModelId    string
	snsTopicArn       string
	metricsName       []string
}

var (
	logger *slog.Logger
)

func init() {
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)
}

func getEnvironmentVariables(varName string) (string, error) {
	varValue, exists := os.LookupEnv(varName)
	if exists {
		return varValue, nil
	} else {
		logger.Error(varName + " environment variable is not set")
		return "", errors.New("missing required environment variable " + varName)
	}
}

func handler(ctx context.Context, event json.RawMessage) (string, error) {
	logger.Info("Initializing Function...")

	// Get Environment Variables
	var ev envValues
	var err error
	ev.dynamoDbTableName, err = getEnvironmentVariables("DYNAMODB_TABLE_NAME")
	if err != nil {
		return "", err
	}
	ev.dynamoDbRegion, err = getEnvironmentVariables("DYNAMODB_REGION")
	if err != nil {
		return "", err
	}
	ev.bedrockRegion, err = getEnvironmentVariables("BEDROCK_REGION")
	if err != nil {
		return "", err
	}
	ev.bedrockModelId, err = getEnvironmentVariables("BEDROCK_MODEL_ID")
	if err != nil {
		return "", err
	}
	ev.snsTopicArn, err = getEnvironmentVariables("SNS_TOPIC_ARN")
	if err != nil {
		return "", err
	}
	for _, env := range os.Environ() {
		if key, value, found := strings.Cut(env, "="); found && strings.HasPrefix(key, "METRICS_NAME_") {
			if value == "" {
				logger.Error(key + " environment variable is empty")
				return "", errors.New("missing required environment variable " + key)
			}
			ev.metricsName = append(ev.metricsName, value)
		}
	}
	if len(ev.metricsName) == 0 || len(ev.metricsName) > 5 {
		logger.Error("The count of metric names must be between 1 and 5")
		return "", errors.New("the count of metric names must be between 1 and 5")
	}

	response, err := ConverseModel(ev)
	if err != nil {
		logger.Error("Error invoking Bedrock Runtime: " + err.Error())
		return "", err
	}

	// Send message to SNS
	err = SendMessageSns(ev, response)
	if err != nil {
		logger.Error("Error sending message to SNS: " + err.Error())
		return "", err
	}

	// End of Function
	logger.Info(string(response))
	logger.Info("Function Completed Successfully")
	return response, nil
}

func main() {
	lambda.Start(handler)
}
