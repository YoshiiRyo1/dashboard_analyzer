# dashboard_analyzer

Dashboard metrics analysis using AWS Bedrock

## Overview

`dashboard_analyzer` is a tool for analyzing dashboard metrics using AWS Bedrock. This tool retrieves parameters from DynamoDB, obtains metrics data from CloudWatch for analysis, and sends the results to an SNS topic.

## File Structure

- `main.go`: Entry point for the Lambda function. Retrieves environment variables, calls the Bedrock model, and sends SNS messages.
- `bedrock.go`: Contains functions for interacting with the Bedrock model.
- `awsModules.go`: Contains functions for retrieving parameters from DynamoDB and obtaining metrics data from CloudWatch using the AWS SDK.
- `compose.yml`: Docker Compose file for local development and testing.
- `Dockerfile`: File for building the Docker image.

## Usage

### Prerequisites

- Docker installed
- AWS account with appropriate permissions
- Add access to foundation models that you want to use
  - ref: [Access Amazon Bedrock foundation models](https://docs.aws.amazon.com/bedrock/latest/userguide/model-access.html)
- Create an ECR repository
  - ref: [Creating a repository](https://docs.aws.amazon.com/AmazonECR/latest/userguide/repository-create.html)
- Create a DynamoDB table
  - ref: [Creating a table](https://docs.aws.amazon.com/amazondynamodb/latest/developerguide/getting-started-step-1.html)
  - Partition key: `awsService` (String)
- Create a Lambda function
  - ref: [Creating a Lambda function](https://docs.aws.amazon.com/lambda/latest/dg/getting-started-create-function.html)
  - Timeout: 30 seconds or more
- Create an SNS topic and set up AWS Chatbot, if you want to send metrics data to Slack or others
  - ref: [Creating a topic](https://docs.aws.amazon.com/sns/latest/dg/sns-getting-started.html)
  - ref: [Getting started with AWS Chatbot](https://docs.aws.amazon.com/chatbot/latest/adminguide/getting-started.html)

### Environment Variables

| Variable Name       | Required | Description                                                                                           |
| ------------------- | -------- | ----------------------------------------------------------------------------------------------------- |
| DYNAMODB_TABLE_NAME | Required | DynamoDB table name to store parameters                                                               |
| DYNAMODB_REGION     | Required | Region where the Lambda function and DynamoDB are located                                             |
| BEDROCK_REGION      | Required | Region where the Bedrock model is located                                                             |
| BEDROCK_MODEL_ID    | Required | Bedrock model ID to use                                                                               |
| SNS_TOPIC_ARN       | Required | ARN of the SNS topic to send metrics data, enter `none` if not sending SNS                            |
| METRICS_NAME_1      | Required | Name of the metrics data, refer to the items in [DynamoDB Table Structure](#dynamodb-table-structure) |
| METRICS_NAME_2      | Optional | Name of the metrics data, up to 5                                                                     |
| METRICS_NAME_3      | Optional | Name of the metrics data, up to 5                                                                     |
| METRICS_NAME_4      | Optional | Name of the metrics data, up to 5                                                                     |
| METRICS_NAME_5      | Optional | Name of the metrics data, up to 5                                                                     |

### DynamoDB Table Structure

Metrics data is retrieved using the CloudWatch [GetMetricData](https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_GetMetricData.html) API and stored in DynamoDB.

`awsService` specifies the name of any metrics data. It is used in the description of the metrics data given to the model, so it should be a term that clearly indicates what the metrics data is.

`dateRange` specifies how many days of data to retrieve. Metrics data for the specified number of days is obtained from CloudWatch. Ensure it does not exceed the 4.5MB size limit.

For other values, refer to [MetricStat](https://docs.aws.amazon.com/AmazonCloudWatch/latest/APIReference/API_MetricStat.html) and the user guides of each service to specify the desired metrics data.

Below is an example of retrieving the RequestCount for ALB. 

```json
{
  "awsService": {
    "S": "ALB_REQUEST_COUNT"
  },
  "dateRange": {
    "S": "3"
  },
  "dimensions": {
    "S": "[{\"LoadBalancer\": \"app/app-alb-unstable/17709a8bf6156327\"}]"
  },
  "metricName": {
    "S": "RequestCount"
  },
  "namespace": {
    "S": "AWS/ApplicationELB"
  },
  "period": {
    "S": "300"
  },
  "stat": {
    "S": "Sum"
  },
  "unit": {
    "S": "Count"
  }
}
```

Below is an example of retrieving the Cpu Utilized for ECS.

```json
{
  "awsService": {
    "S": "ECS_CPU_UTILIZED"
  },
  "dateRange": {
    "S": "3"
  },
  "dimensions": {
    "S": "[{\"ClusterName\": \"cluster-unstable\"},{\"ServiceName\": \"app-service-unstable\"}]"
  },
  "metricName": {
    "S": "CpuUtilized"
  },
  "namespace": {
    "S": "ECS/ContainerInsights"
  },
  "period": {
    "S": "300"
  },
  "stat": {
    "S": "p90"
  },
  "unit": {
    "S": "None"
  }
}
```

### How to run locally

To verify operation or debug on your local PC, follow these steps:

```bash
# Register AWS credentials in the .env file, example below
env | grep AWS > .env

docker compose build
docker compose up -d

curl "http://localhost:9000/2015-03-31/functions/function/invocations" -d '{}'
```

### Deployment

To deploy to AWS Lambda, push the built Docker image to ECR and set it as a Lambda function.   

