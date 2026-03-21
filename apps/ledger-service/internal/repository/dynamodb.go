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
	"github.com/google/uuid"
)

type LedgerEntry struct {
	EntryID     string    `dynamodbav:"entry_id"`
	PaymentID   string    `dynamodbav:"payment_id"`
	EventType   string    `dynamodbav:"event_type"`
	Amount      float64   `dynamodbav:"amount"`
	UserID      string    `dynamodbav:"user_id"`
	Description string    `dynamodbav:"description"`
	Timestamp   time.Time `dynamodbav:"timestamp"`
}

type DynamoDBRepository struct {
	client      *dynamodb.Client
	ledgerTable string
}

func NewDynamoDBRepository(ctx context.Context, region, ledgerTable string) (*DynamoDBRepository, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &DynamoDBRepository{
		client:      dynamodb.NewFromConfig(cfg),
		ledgerTable: ledgerTable,
	}, nil
}

func (r *DynamoDBRepository) RecordEntry(ctx context.Context, entry *LedgerEntry) error {
	entry.EntryID = uuid.New().String()
	entry.Timestamp = time.Now()

	av, err := attributevalue.MarshalMap(entry)
	if err != nil {
		return err
	}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.ledgerTable),
		Item:      av,
	})

	return err
}

func (r *DynamoDBRepository) GetEntries(ctx context.Context, limit int) ([]*LedgerEntry, error) {
	result, err := r.client.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(r.ledgerTable),
		Limit:     aws.Int32(int32(limit)),
	})

	if err != nil {
		return nil, err
	}

	var entries []*LedgerEntry
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &entries); err != nil {
		return nil, err
	}

	return entries, nil
}

func (r *DynamoDBRepository) GetEntriesByPayment(ctx context.Context, paymentID string) ([]*LedgerEntry, error) {
	result, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.ledgerTable),
		KeyConditionExpression: aws.String("payment_id = :payment_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":payment_id": &types.AttributeValueMemberS{Value: paymentID},
		},
	})

	if err != nil {
		return nil, err
	}

	var entries []*LedgerEntry
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &entries); err != nil {
		return nil, err
	}

	return entries, nil
}
