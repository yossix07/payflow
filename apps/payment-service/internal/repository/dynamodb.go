package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/payflow/payment-service/internal/model"
)

type DynamoDBRepository struct {
	client           *dynamodb.Client
	paymentsTable    string
	idempotencyTable string
}

func NewDynamoDBRepository(ctx context.Context, region, paymentsTable, idempotencyTable string) (*DynamoDBRepository, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &DynamoDBRepository{
		client:           dynamodb.NewFromConfig(cfg),
		paymentsTable:    paymentsTable,
		idempotencyTable: idempotencyTable,
	}, nil
}

func (r *DynamoDBRepository) SavePayment(ctx context.Context, payment *model.Payment) error {
	av, err := attributevalue.MarshalMap(payment)
	if err != nil {
		return fmt.Errorf("failed to marshal payment: %w", err)
	}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.paymentsTable),
		Item:      av,
	})

	return err
}

func (r *DynamoDBRepository) GetPayment(ctx context.Context, paymentID string) (*model.Payment, error) {
	result, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.paymentsTable),
		Key: map[string]types.AttributeValue{
			"payment_id": &types.AttributeValueMemberS{Value: paymentID},
		},
	})

	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, fmt.Errorf("payment not found: %s", paymentID)
	}

	var payment model.Payment
	if err := attributevalue.UnmarshalMap(result.Item, &payment); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payment: %w", err)
	}

	return &payment, nil
}

func (r *DynamoDBRepository) GetStuckPayments(ctx context.Context, state model.PaymentState, olderThan time.Time) ([]*model.Payment, error) {
	result, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.paymentsTable),
		IndexName:              aws.String("state-updated-index"),
		KeyConditionExpression: aws.String("#state = :state AND updated_at < :threshold"),
		ExpressionAttributeNames: map[string]string{
			"#state": "state",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":state":     &types.AttributeValueMemberS{Value: string(state)},
			":threshold": &types.AttributeValueMemberS{Value: olderThan.Format(time.RFC3339)},
		},
		Limit: aws.Int32(25),
	})
	if err != nil {
		return nil, err
	}

	var payments []*model.Payment
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &payments); err != nil {
		return nil, err
	}
	return payments, nil
}

func (r *DynamoDBRepository) ConditionalUpdateState(ctx context.Context, paymentID string, expectedState, newState model.PaymentState) error {
	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.paymentsTable),
		Key: map[string]types.AttributeValue{
			"payment_id": &types.AttributeValueMemberS{Value: paymentID},
		},
		UpdateExpression:    aws.String("SET #state = :new_state, updated_at = :now"),
		ConditionExpression: aws.String("#state = :expected_state"),
		ExpressionAttributeNames: map[string]string{
			"#state": "state",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":new_state":      &types.AttributeValueMemberS{Value: string(newState)},
			":expected_state": &types.AttributeValueMemberS{Value: string(expectedState)},
			":now":            &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
		},
	})
	return err
}

type IdempotencyRecord struct {
	Key        string `dynamodbav:"idempotency_key"`
	Response   string `dynamodbav:"response"`
	StatusCode int    `dynamodbav:"status_code"`
	CreatedAt  string `dynamodbav:"created_at"`
	ExpiresAt  int64  `dynamodbav:"expires_at"` // TTL
}

func (r *DynamoDBRepository) CheckIdempotency(ctx context.Context, key string) (string, int, bool, error) {
	result, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.idempotencyTable),
		Key: map[string]types.AttributeValue{
			"idempotency_key": &types.AttributeValueMemberS{Value: key},
		},
	})

	if err != nil {
		return "", 0, false, err
	}

	if result.Item == nil {
		return "", 0, false, nil
	}

	var record IdempotencyRecord
	if err := attributevalue.UnmarshalMap(result.Item, &record); err != nil {
		return "", 0, false, err
	}

	return record.Response, record.StatusCode, true, nil
}

func (r *DynamoDBRepository) SaveIdempotency(ctx context.Context, key string, response string, statusCode int) error {
	record := IdempotencyRecord{
		Key:        key,
		Response:   response,
		StatusCode: statusCode,
		CreatedAt:  time.Now().Format(time.RFC3339),
		ExpiresAt:  time.Now().Add(24 * time.Hour).Unix(), // 24 hour TTL
	}

	av, err := attributevalue.MarshalMap(record)
	if err != nil {
		return err
	}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.idempotencyTable),
		Item:      av,
	})

	return err
}
