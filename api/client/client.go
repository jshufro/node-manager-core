package client

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/goccy/go-json"
	"github.com/rocket-pool/node-manager-core/api/types"
	"github.com/rocket-pool/node-manager-core/beacon"
	"github.com/rocket-pool/node-manager-core/log"

	"github.com/ethereum/go-ethereum/common"
)

const (
	jsonContentType string = "application/json"
)

// IRequester is an interface for making HTTP requests to a specific subroute on the NMC server
type IRequester interface {
	// The human-readable requester name (for logging)
	GetName() string

	// The name of the subroute to send requests to
	GetRoute() string

	// Context to hold settings and utilities the requester should use
	GetContext() *RequesterContext
}

// Submit a GET request to the API server
func SendGetRequest[DataType any](r IRequester, method string, requestName string, args map[string]string) (*types.ApiResponse[DataType], error) {
	if args == nil {
		args = map[string]string{}
	}
	response, err := RawGetRequest[DataType](r.GetContext(), fmt.Sprintf("%s/%s", r.GetRoute(), method), args)
	if err != nil {
		return nil, fmt.Errorf("error during %s %s request: %w", r.GetName(), requestName, err)
	}
	return response, nil
}

// Submit a GET request to the API server
func RawGetRequest[DataType any](context *RequesterContext, path string, params map[string]string) (*types.ApiResponse[DataType], error) {
	// Make sure the socket exists
	_, err := os.Stat(context.socketPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("the socket at [%s] does not exist - please start the service and try again", context.socketPath)
	}

	// Create the request
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s/%s", context.base, path), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating HTTP request: %w", err)
	}

	// Encode the params into a query string
	values := url.Values{}
	for name, value := range params {
		values.Add(name, value)
	}
	req.URL.RawQuery = values.Encode()

	// Debug log
	context.logger.Debug("API Request", slog.String(log.MethodKey, http.MethodGet), slog.String(log.QueryKey, req.URL.String()))

	// Run the request
	resp, err := context.client.Do(req)
	return HandleResponse[DataType](context, resp, path, err)
}

// Submit a POST request to the API server
func SendPostRequest[DataType any](r IRequester, method string, requestName string, body any) (*types.ApiResponse[DataType], error) {
	// Serialize the body
	bytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("error serializing request body for %s %s: %w", r.GetName(), requestName, err)
	}

	response, err := RawPostRequest[DataType](r.GetContext(), fmt.Sprintf("%s/%s", r.GetRoute(), method), string(bytes))
	if err != nil {
		return nil, fmt.Errorf("error during %s %s request: %w", r.GetName(), requestName, err)
	}
	return response, nil
}

// Submit a POST request to the API server
func RawPostRequest[DataType any](context *RequesterContext, path string, body string) (*types.ApiResponse[DataType], error) {
	// Make sure the socket exists
	_, err := os.Stat(context.socketPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("the socket at [%s] does not exist - please start the service and try again", context.socketPath)
	}

	// Debug log
	context.logger.Debug("API Request", slog.String(log.MethodKey, http.MethodPost), slog.String(log.PathKey, path), slog.String(log.BodyKey, body))

	resp, err := context.client.Post(fmt.Sprintf("http://%s/%s", context.base, path), jsonContentType, strings.NewReader(body))
	return HandleResponse[DataType](context, resp, path, err)
}

// Processes a response to a request
func HandleResponse[DataType any](context *RequesterContext, resp *http.Response, path string, err error) (*types.ApiResponse[DataType], error) {
	if err != nil {
		return nil, fmt.Errorf("error requesting %s: %w", path, err)
	}

	// Read the body
	defer resp.Body.Close()
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading the response body for %s: %w", path, err)
	}

	// Handle 404s specially since they won't have a JSON body
	if resp.StatusCode == http.StatusNotFound {
		context.logger.Debug("API Response (raw)", slog.String(log.CodeKey, resp.Status), slog.String(log.BodyKey, string(bytes)))
		return nil, fmt.Errorf("route '%s' not found", path)
	}

	// Deserialize the response into the provided type
	var parsedResponse types.ApiResponse[DataType]
	err = json.Unmarshal(bytes, &parsedResponse)
	if err != nil {
		context.logger.Debug("API Response (raw)", slog.String(log.CodeKey, resp.Status), slog.String(log.BodyKey, string(bytes)))
		return nil, fmt.Errorf("error deserializing response to %s: %w", path, err)
	}

	// Check if the request failed
	if resp.StatusCode != http.StatusOK {
		context.logger.Debug("API Response", slog.String(log.PathKey, path), slog.String(log.CodeKey, resp.Status), slog.String("err", parsedResponse.Error))
		return nil, fmt.Errorf(parsedResponse.Error)
	}

	// Debug log
	context.logger.Debug("API Response", slog.String(log.BodyKey, string(bytes)))

	return &parsedResponse, nil
}

// Types that can be batched into a comma-delmited string
type BatchInputType interface {
	uint64 | common.Address | beacon.ValidatorPubkey
}

// Converts an array of inputs into a comma-delimited string
func MakeBatchArg[DataType BatchInputType](input []DataType) string {
	results := make([]string, len(input))

	// Figure out how to stringify the input
	switch typedInput := any(&input).(type) {
	case *[]uint64:
		for i, index := range *typedInput {
			results[i] = strconv.FormatUint(index, 10)
		}
	case *[]common.Address:
		for i, address := range *typedInput {
			results[i] = address.Hex()
		}
	case *[]beacon.ValidatorPubkey:
		for i, pubkey := range *typedInput {
			results[i] = pubkey.HexWithPrefix()
		}
	}
	return strings.Join(results, ",")
}
