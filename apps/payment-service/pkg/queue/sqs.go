package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type SQSClient struct {
	client   *sqs.Client
	queueURL string
}

func NewSQSClient(ctx context.Context, region, queueURL string) (*SQSClient, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &SQSClient{
		client:   sqs.NewFromConfig(cfg),
		queueURL: queueURL,
	}, nil
}

func (c *SQSClient) SendMessage(ctx context.Context, body string) error {
	_, err := c.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(c.queueURL),
		MessageBody: aws.String(body),
	})
	return err
}

func (c *SQSClient) ReceiveMessages(ctx context.Context) ([]Message, error) {
	result, err := c.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(c.queueURL),
		MaxNumberOfMessages: 10,
		WaitTimeSeconds:     20, // Long polling
	})

	if err != nil {
		return nil, err
	}

	messages := make([]Message, 0, len(result.Messages))
	for _, msg := range result.Messages {
		// Parse message to extract event type
		var wrapper map[string]interface{}
		if err := json.Unmarshal([]byte(*msg.Body), &wrapper); err != nil {
			continue
		}

		eventType, _ := wrapper["event_type"].(string)
		payload, _ := json.Marshal(wrapper["payload"])

		messages = append(messages, Message{
			MessageID:     *msg.MessageId,
			ReceiptHandle: *msg.ReceiptHandle,
			EventType:     eventType,
			Body:          string(payload),
		})
	}

	return messages, nil
}

func (c *SQSClient) DeleteMessage(ctx context.Context, receiptHandle string) error {
	_, err := c.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(c.queueURL),
		ReceiptHandle: aws.String(receiptHandle),
	})
	return err
}
