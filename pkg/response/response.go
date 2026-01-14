package response

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/beckn-one/beckn-onix/pkg/log"
	"github.com/beckn-one/beckn-onix/pkg/model"
)

func SendBody(ctx context.Context, w http.ResponseWriter, body interface{}) {

	if bodyStr, ok := body.(string); ok {
		body = parseJSONOrDefault(bodyStr)
	}

	data, err := json.Marshal(body)
	if err != nil {
		log.Errorf(ctx,err,"Failed to marshal response body, MessageID: %s", ctx.Value(model.ContextKeyMsgID))
		http.Error(w, fmt.Sprintf("Internal server error, MessageID: %s", ctx.Value(model.ContextKeyMsgID)), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, er := w.Write(data)
	if er != nil {
		log.Errorf(ctx,er,"Error writing response: %v, MessageID: %s", er, ctx.Value(model.ContextKeyMsgID))
		http.Error(w, fmt.Sprintf("Internal server error, MessageID: %s", ctx.Value(model.ContextKeyMsgID)), http.StatusInternalServerError)
		return
	}
}

// ParseJSONOrDefault attempts to parse a JSON string into an interface{}.
// If parsing fails, it returns a map with the original string as a message.
func parseJSONOrDefault(str string) interface{} {
	var result interface{}
	
	if err := json.Unmarshal([]byte(str), &result); err != nil {
		// JSON parsing failed, return default structure
		return map[string]interface{}{
			"message": str,
		}
	}
	
	return result
}


// SendAck sends an acknowledgment response (ACK) to the client.
func SendAck(w http.ResponseWriter) {
	log.Infof(context.Background(),"Sending Ack")
	resp := &model.Response{
		Message: model.Message{
			Ack: model.Ack{
				Status: model.StatusACK,
			},
		},
	}

	data, _ := json.Marshal(resp) //should not fail here

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write(data)
	if err != nil {
		http.Error(w, "failed to write response", http.StatusInternalServerError)
		return
	}
	log.Infof(context.Background(),"Ack sent successfully")
}

// nack sends a negative acknowledgment (NACK) response with an error message.
func nack(ctx context.Context, w http.ResponseWriter, err *model.Error, status int) {
	log.Infof(ctx,"Sending Nack: code %s, message %s", err.Code, err.Message)
	resp := &model.Response{
		Message: model.Message{
			Ack: model.Ack{
				Status: model.StatusNACK,
			},
		},
		Error: &model.Error{
			Code:    err.Code,
			Message: err.Message,
		},
	}
	if(err.Context != nil){
		resp.Context = err.Context
	}
	if(err.Code == "500"){
		resp.Message.Error.Message = "INTERNAL_SERVER_ERROR"
	}

	data, _ := json.Marshal(resp) //should not fail here

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, er := w.Write(data)
	if er != nil {
		log.Debugf(ctx, "Error writing response: %v, MessageID: %s", er, ctx.Value(model.ContextKeyMsgID))
		http.Error(w, fmt.Sprintf("Internal server error, MessageID: %s", ctx.Value(model.ContextKeyMsgID)), http.StatusInternalServerError)
		return
	}
}

// internalServerError generates an internal server error response.
func internalServerError(ctx context.Context) *model.Error {
	return &model.Error{
		Code:    http.StatusText(http.StatusInternalServerError),
		Message: fmt.Sprintf("Internal server error, MessageID: %s", ctx.Value(model.ContextKeyMsgID)),
	}
}

// SendNack processes different types of errors and sends an appropriate NACK response.
func SendNack(ctx context.Context, w http.ResponseWriter, err error) {
	var schemaErr *model.SchemaValidationErr
	var signErr *model.SignValidationErr
	var badReqErr *model.BadReqErr
	var notFoundErr *model.NotFoundErr
	var workbenchErr *model.WorkbenchErr

	log.Errorf(ctx,err,"Responding Error")

	switch {
	case errors.As(err, &workbenchErr):
		behavior := workbenchErr.Behavior
		switch behavior {
		case "NACK":
			nack(ctx, w, workbenchErr.BecknError(), 200)
			return
		case "HTTP":
			code, _ := strconv.Atoi(workbenchErr.Err.Code)
			nack(ctx, w, workbenchErr.BecknError(), code)
			return
		}
	case errors.As(err, &schemaErr):
		nack(ctx, w, schemaErr.BecknError(), 200)
		return
	case errors.As(err, &signErr):
		nack(ctx, w, signErr.BecknError(), http.StatusUnauthorized)
		return
	case errors.As(err, &badReqErr):
		nack(ctx, w, badReqErr.BecknError(), http.StatusBadRequest)
		return
	case errors.As(err, &notFoundErr):
		nack(ctx, w, notFoundErr.BecknError(), http.StatusNotFound)
		return
	default:
		nack(ctx, w, internalServerError(ctx), http.StatusInternalServerError)
		return
	}
}