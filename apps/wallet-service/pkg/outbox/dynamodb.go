package outbox

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

type DynamoDBOutbox struct {
	client    *dynamodb.Client
	tableName string
}

func NewDynamoDBOutbox(ctx context.Context, region, tableName string) (*DynamoDBOutbox, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &DynamoDBOutbox{
		client:    dynamodb.NewFromConfig(cfg),
		tableName: tableName,
	}, nil
}

func (o *DynamoDBOutbox) WriteMessage(ctx context.Context, eventType string, payload string) error {
	msg := OutboxMessage{
		MessageID: uuid.New().String(),
		EventType: eventType,
		Payload:   payload,
		Published: 0,
		CreatedAt: time.Now().Format(time.RFC3339),
	}

	av, err := attributevalue.MarshalMap(msg)
	if err != nil {
		return err
	}

	_, err = o.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(o.tableName),
		Item:      av,
	})

	return err
}

func (o *DynamoDBOutbox) GetUnpublishedMessages(ctx context.Context) ([]OutboxMessage, error) {
	result, err := o.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(o.tableName),
		IndexName:              aws.String("published-index"),
		KeyConditionExpression: aws.String("published = :published"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":published": &types.AttributeValueMemberN{Value: "0"},
		},
		Limit: aws.Int32(10),
	})

	if err != nil {
		return nil, err
	}

	var messages []OutboxMessage
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &messages); err != nil {
		return nil, err
	}

	return messages, nil
}

func (o *DynamoDBOutbox) MarkAsPublished(ctx context.Context, messageID string) error {
	_, err := o.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(o.tableName),
		Key: map[string]types.AttributeValue{
			"message_id": &types.AttributeValueMemberS{Value: messageID},
		},
		UpdateExpression: aws.String("SET published = :published"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":published": &types.AttributeValueMemberN{Value: "1"},
		},
	})

	return err
}

func (o *DynamoDBOutbox) MarkAsFailed(ctx context.Context, messageID string) error {
	_, err := o.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(o.tableName),
		Key: map[string]types.AttributeValue{
			"message_id": &types.AttributeValueMemberS{Value: messageID},
		},
		UpdateExpression: aws.String("SET published = :failed"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":failed": &types.AttributeValueMemberN{Value: "-1"},
		},
	})
	return err
}

func (o *DynamoDBOutbox) IncrementRetryCount(ctx context.Context, messageID string) error {
	_, err := o.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(o.tableName),
		Key: map[string]types.AttributeValue{
			"message_id": &types.AttributeValueMemberS{Value: messageID},
		},
		UpdateExpression: aws.String("SET retry_count = if_not_exists(retry_count, :zero) + :one"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":zero": &types.AttributeValueMemberN{Value: "0"},
			":one":  &types.AttributeValueMemberN{Value: "1"},
		},
	})
	return err
}
