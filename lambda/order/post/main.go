package main

import (
	"encoding/json"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/knoebber/comptche-shop/lambda/util"
	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/order"
)

type orderResponse struct {
	OrderID string `json:"orderID"`
	Message string `json:"message"`
	Target  string `json:"target"`
}

// HandleRequest processes a lambda request.
func HandleRequest(request events.APIGatewayProxyRequest) (response events.APIGatewayProxyResponse, err error) {
	var (
		requestBody  stripe.OrderParams
		responseBody orderResponse
		o            *stripe.Order
	)

	// TODO only cosmostuna.com
	response.Headers = map[string]string{"Access-Control-Allow-Origin": "*"}

	if err = json.Unmarshal([]byte(request.Body), &requestBody); err != nil {
		response.StatusCode = 400
		return
	}

	if !shippable(requestBody.Shipping.Address.State) {
		responseBody.Message = "We only ship to the lower 48 US states."
		responseBody.Target = "state"
		util.SetResponseBody(&response, &responseBody)
		return
	}

	if err = util.SetStripeKey(request.RequestContext.Stage); err != nil {
		response.StatusCode = 500
		return
	}

	requestBody.Currency = stripe.String(string(stripe.CurrencyUSD))
	requestBody.Items[0].Type = stripe.String(string(stripe.OrderItemTypeSKU))
	requestBody.Shipping.Address.Country = stripe.String("US")

	var validOrder bool
	for _, offer := range util.BulkOffers {
		// Only accept orders that match one of our offers.
		if offer.Quantity == *requestBody.Items[0].Quantity {
			requestBody.Coupon = offer.CouponID
			validOrder = true
		}
	}
	if !validOrder {
		responseBody.Message = "Order is invalid."
		responseBody.Target = "product-grid"
		util.SetResponseBody(&response, &responseBody)
		return
	}

	o, err = order.New(&requestBody)
	if err != nil {
		response.StatusCode = 500
		return
	}
	responseBody.OrderID = o.ID

	util.SetResponseBody(&response, &responseBody)
	return
}

func main() {
	lambda.Start(HandleRequest)
}
