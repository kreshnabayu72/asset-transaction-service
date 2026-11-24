package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	marhttp "gitea.tagsamurai.local/BETS-V2/service-contracts/services/master_asset_registry/http"
	tsConstant "gitea.tagsamurai.local/BETS-V2/ts-utils/v2/constant"
)

// requestTimeout bounds one internal call, so a slow owner service fails the request instead of holding it.
const requestTimeout = 10 * time.Second

// maxLookupIDs is the id cap of one master-asset-registry lookup request.
const maxLookupIDs = marhttp.MaxInternalLookupIDs

var (
	// errMissingData reports an internal answer without the data member of the helper envelope.
	errMissingData = errors.New("response has no data")
	// errContract reports an owner-service value outside the vocabulary the module reads.
	errContract = errors.New("unexpected value from owner service")
)

// statusError reports a non-2xx answer of an owner service.
type statusError struct {
	service string
	path    string
	status  int
}

func (e *statusError) Error() string {
	return fmt.Sprintf("%s %s answered HTTP %d", e.service, e.path, e.status)
}

// internalClient calls the internal routes of one owner service with the service token and the tenant.
type internalClient struct {
	service      string
	baseURL      string
	serviceToken string
	client       *http.Client
}

func newInternalClient(service, baseURL, serviceToken string) internalClient {
	return internalClient{
		service:      service,
		baseURL:      strings.TrimRight(baseURL, "/"),
		serviceToken: serviceToken,
		client:       &http.Client{Timeout: requestTimeout},
	}
}

// call sends one request and returns the body of a 2xx answer.
func (c internalClient) call(ctx context.Context, method, path, clientID string, query url.Values, payload any) ([]byte, error) {
	target := c.baseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode %s request %s: %w", c.service, path, err)
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, fmt.Errorf("build %s request %s: %w", c.service, path, err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set(tsConstant.HEADER_SERVICE_TOKEN, c.serviceToken)
	req.Header.Set(tsConstant.HEADER_CLIENT_ID, clientID)
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call %s %s: %w", c.service, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read %s %s: %w", c.service, path, err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, &statusError{service: c.service, path: path, status: resp.StatusCode}
	}
	return raw, nil
}

// decodeData decodes the data member of a helper.Success envelope into out.
func decodeData(raw []byte, out any) error {
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decode envelope: %w", err)
	}
	if len(envelope.Data) == 0 {
		return errMissingData
	}
	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("decode data: %w", err)
	}
	return nil
}

// chunkIDs splits ids into lookup requests of at most maxLookupIDs ids each.
func chunkIDs(ids []int64) [][]int64 {
	var chunks [][]int64
	for start := 0; start < len(ids); start += maxLookupIDs {
		chunks = append(chunks, ids[start:min(start+maxLookupIDs, len(ids))])
	}
	return chunks
}

// idStrings formats ids as the decimal strings the lookup contracts take.
func idStrings(ids []int64) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = strconv.FormatInt(id, 10)
	}
	return out
}

// parseID reads a decimal id of an owner-service answer.
func parseID(raw string) (int64, error) {
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: id %q", errContract, raw)
	}
	return id, nil
}

// parseIDPtr reads an optional decimal id; nil stays nil.
func parseIDPtr(raw *string) (*int64, error) {
	if raw == nil {
		return nil, nil
	}
	id, err := parseID(*raw)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// indexByID maps each item by the id its key names; a malformed id fails the whole answer.
func indexByID[T any](items []T, key func(T) string) (map[int64]T, error) {
	out := make(map[int64]T, len(items))
	for _, item := range items {
		id, err := parseID(key(item))
		if err != nil {
			return nil, err
		}
		out[id] = item
	}
	return out, nil
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefBool(b *bool) bool {
	return b != nil && *b
}
