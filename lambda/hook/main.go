package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ses"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/knoebber/comptche-shop/lambda/util"
	"github.com/pkg/errors"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/webhook"
)

const (
	sender     = "mail@cosmostuna.com"
	charSet    = "UTF-8"
	snsSubject = "Cosmo's Tuna: New Paid Order"
)

var sess *session.Session

type eventResponse struct {
	Message string `json: "message"`
}

func buildEvent(requestBody []byte, signature, secret string) (*stripe.Event, error) {
	// Pass the request body and Stripe-Signature header to ConstructEvent, along
	// with the webhook signing key.
	event, err := webhook.ConstructEvent(requestBody, signature, secret)

	if err != nil {
		return nil, errors.Errorf("constructing event, %v", err)
	}
	return &event, nil
}

// Publishes a message to a SNS (Simple Notification Service) topic.
// Used for notifying us when a new order is paid.
func publishMessage(stage, orderID string) {
	var (
		stripeLink  string
		confirmLink string
		subject     string
	)

	// Don't publish test messages for now.
	if stage != "prod" {
		return
	}

	snsTopicArn := os.Getenv("sns_topic_arn")
	if snsTopicArn == "" {
		log.Print("must set sns_topic_arn env value to publish messages")
		return
	}

	if stage == "prod" {
		stripeLink = fmt.Sprintf("https://dashboard.stripe.com/orders/%s", orderID)
		confirmLink = fmt.Sprintf("https://cosmostuna.com/confirm.html?order=%s", orderID)
		subject = snsSubject
	} else {
		stripeLink = fmt.Sprintf("https://dashboard.stripe.com/test/orders/%s", orderID)
		confirmLink = fmt.Sprintf("https://cosmostuna.com/dev-confirm.html?order=%s", orderID)
		subject = "[TEST MODE] " + snsSubject
	}

	msg := fmt.Sprintf("Stripe: %s\nOrder Confirm: %s", stripeLink, confirmLink)

	client := sns.New(sess)

	input := &sns.PublishInput{
		TopicArn: aws.String(snsTopicArn),
		Subject:  aws.String(subject),
		Message:  aws.String(msg),
	}

	// The second parameter is sns.PublishOutput
	req, _ := client.PublishRequest(input)
	if err := req.Send(); err != nil {
		log.Printf("failed to publish SNS message: %v\n", err)
		return
	}

	log.Printf("Published to SNS topic")
}

// HandleRequest processes a lambda request.
func HandleRequest(request events.APIGatewayProxyRequest) (response events.APIGatewayProxyResponse, err error) {
	var (
		responseBody eventResponse
		event        *stripe.Event
		o            stripe.Order
		subject      string
		body         string
		secret       string
		quantityStr  string
		quantity     int64
	)

	sess, err = session.NewSession(&aws.Config{Region: aws.String(util.AWSRegion)})
	if err != nil {
		response.StatusCode = 500
		return
	}

	secret, err = util.HookSecret(request.RequestContext.Stage)
	if err != nil {
		response.StatusCode = 500
		return
	}

	signature, ok := request.Headers["Stripe-Signature"]
	if !ok {
		err = errors.New("stripe signature header missing")
		response.StatusCode = 400
		return
	}

	event, err = buildEvent([]byte(request.Body), signature, secret)
	if err != nil {
		response.StatusCode = 400
		return
	}

	err = json.Unmarshal(event.Data.Raw, &o)
	if err != nil {
		err = fmt.Errorf("parsing webhook JSON, %v", err)
		response.StatusCode = 500
		return
	}

	for _, item := range o.Items {
		if item.Type == stripe.OrderItemTypeSKU {
			quantity = item.Quantity
			break
		}
	}
	if quantity > 1 {
		quantityStr = fmt.Sprintf("%d cans of tuna", quantity)
	} else {
		quantityStr = "tuna can"
	}

	switch stripe.OrderStatus(o.Status) {
	case stripe.OrderStatusPaid:
		publishMessage(request.RequestContext.Stage, o.ID)
		subject = "Your Cosmo's Tuna order has processed"
		body = fmt.Sprintf("Thank you for your order! We’ll send a confirmation when your %s ships.", quantityStr)
	case stripe.OrderStatusFulfilled:
		subject = "Your Cosmo's Tuna order has shipped"
		body = fmt.Sprintf("Your %s will arrive soon.", quantityStr)
	case stripe.OrderStatusCanceled:
		subject = "Your Cosmo's Tuna order has been canceled"
		body = "You will be refunded."
	default:
		responseBody.Message = fmt.Sprintf("unknown order status %#v", o.Status)
		util.SetResponseBody(&response, &responseBody)
		return
	}

	if err = sendEmail(o.Email, o.ID, subject, body, o.Shipping.TrackingNumber); err != nil {
		// Don't throw 500's on email errors.
		responseBody.Message = fmt.Sprintf("failed to email %s: %v", o.Email, err)
	} else {
		responseBody.Message = fmt.Sprintf("emailed %s, order status is %s", o.Email, o.Status)
	}

	util.SetResponseBody(&response, &responseBody)
	return
}

func sendEmail(address, orderID, subject, body, tracking string) error {

	// Create a SES session.
	svc := ses.New(sess)

	htmlBody := fmt.Sprintf(`
<p>%s</p>
<a href="https://www.cosmostuna.com/confirm.html?order=%s">Review your order here.</a>
`,
		body, orderID)
	if tracking != "" {
		htmlBody += fmt.Sprintf(`
<p>Tracking: 
  <a href="https://tools.usps.com/go/TrackConfirmAction?tLabels=%s" target="_blank">%s</a>
  (USPS)
</p>`, tracking, tracking)

		// For plain text emails.
		body += "\n USPS tracking number: " + tracking
	}
	htmlBody += `
<p>Please <a href="https://www.cosmostuna.com/about.html">contact us</a> if you have any questions.</p>`

	// Assemble the email.
	input := &ses.SendEmailInput{
		Destination: &ses.Destination{
			ToAddresses: []*string{aws.String(address)},
		},
		Message: &ses.Message{
			Body: &ses.Body{
				Html: &ses.Content{
					Charset: aws.String(charSet),
					Data:    aws.String(htmlBody),
				},
				Text: &ses.Content{
					Charset: aws.String(charSet),
					Data:    aws.String(body),
				},
			},
			Subject: &ses.Content{
				Charset: aws.String(charSet),
				Data:    aws.String(subject),
			},
		},
		Source: aws.String(sender),
	}
	if _, err := svc.SendEmail(input); err != nil {
		return fmt.Errorf("failed to send email to %#v, %v", address, err)
	}
	log.Printf("succeeded to send email from %s to %s\n", sender, address)

	return nil
}

func main() {
	lambda.Start(HandleRequest)
}
