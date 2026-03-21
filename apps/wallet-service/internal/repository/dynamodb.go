package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type DynamoDBRepository struct {
	client            *dynamodb.Client
	walletsTable      string
	reservationsTable string
}

func NewDynamoDBRepository(ctx context.Context, region, walletsTable, reservationsTable string) (*DynamoDBRepository, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &DynamoDBRepository{
		client:            dynamodb.NewFromConfig(cfg),
		walletsTable:      walletsTable,
		reservationsTable: reservationsTable,
	}, nil
}

func (r *DynamoDBRepository) GetWallet(ctx context.Context, userID string) (*Wallet, error) {
	result, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.walletsTable),
		Key: map[string]types.AttributeValue{
			"user_id": &types.AttributeValueMemberS{Value: userID},
		},
	})

	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		// Return empty wallet
		return &Wallet{
			UserID:  userID,
			Balance: 0,
		}, nil
	}

	var wallet Wallet
	if err := attributevalue.UnmarshalMap(result.Item, &wallet); err != nil {
		return nil, err
	}

	return &wallet, nil
}

func (r *DynamoDBRepository) CreateWallet(ctx context.Context, wallet *Wallet) error {
	av, err := attributevalue.MarshalMap(wallet)
	if err != nil {
		return err
	}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.walletsTable),
		Item:      av,
	})

	return err
}

func (r *DynamoDBRepository) UpdateWallet(ctx context.Context, wallet *Wallet) error {
	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.walletsTable),
		Key: map[string]types.AttributeValue{
			"user_id": &types.AttributeValueMemberS{Value: wallet.UserID},
		},
		UpdateExpression:    aws.String("SET balance = :balance, updated_at = :updated_at, version = :new_version"),
		ConditionExpression: aws.String("attribute_not_exists(version) OR version = :expected_version"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":balance":          &types.AttributeValueMemberN{Value: fmt.Sprintf("%f", wallet.Balance)},
			":updated_at":       &types.AttributeValueMemberS{Value: wallet.UpdatedAt.Format(time.RFC3339)},
			":new_version":      &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", wallet.Version+1)},
			":expected_version": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", wallet.Version)},
		},
	})

	return err
}

func (r *DynamoDBRepository) CreateReservation(ctx context.Context, reservation *Reservation) error {
	av, err := attributevalue.MarshalMap(reservation)
	if err != nil {
		return err
	}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(r.reservationsTable),
		Item:                av,
		ConditionExpression: aws.String("attribute_not_exists(reservation_id)"),
	})

	// If reservation already exists (from a prior retry), that's OK
	var condErr *types.ConditionalCheckFailedException
	if errors.As(err, &condErr) {
		return nil
	}

	return err
}

func (r *DynamoDBRepository) GetReservationByPayment(ctx context.Context, paymentID string) (*Reservation, error) {
	result, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.reservationsTable),
		IndexName:              aws.String("payment-index"),
		KeyConditionExpression: aws.String("payment_id = :payment_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":payment_id": &types.AttributeValueMemberS{Value: paymentID},
		},
	})

	if err != nil {
		return nil, err
	}

	if len(result.Items) == 0 {
		return nil, fmt.Errorf("reservation not found for payment: %s", paymentID)
	}

	var reservation Reservation
	if err := attributevalue.UnmarshalMap(result.Items[0], &reservation); err != nil {
		return nil, err
	}

	return &reservation, nil
}

func (r *DynamoDBRepository) UpdateReservation(ctx context.Context, reservation *Reservation) error {
	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.reservationsTable),
		Key: map[string]types.AttributeValue{
			"reservation_id": &types.AttributeValueMemberS{Value: reservation.ReservationID},
		},
		UpdateExpression: aws.String("SET #status = :status"),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status": &types.AttributeValueMemberS{Value: reservation.Status},
		},
	})

	return err
}
