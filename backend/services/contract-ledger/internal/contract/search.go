package contract

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

const contractSearchPageSize = 50

type SearchContractsInput struct {
	Keyword  string
	Status   string
	Currency string
	Cursor   string
}

type ContractSearchQuery struct {
	Keyword        string
	Status         Status
	Currency       string
	AfterCreatedAt time.Time
	AfterID        string
	Limit          int
}

type ContractSearchPage struct {
	Contracts  []Contract
	Keyword    string
	Status     Status
	Currency   string
	NextCursor string
}

type contractSearchCursor struct {
	Version   int    `json:"v"`
	CreatedAt string `json:"created_at"`
	ID        string `json:"id"`
	Keyword   string `json:"q"`
	Status    string `json:"status"`
	Currency  string `json:"currency"`
}

func (s *Service) Search(ctx context.Context, input SearchContractsInput) (ContractSearchPage, error) {
	query, err := normalizeContractSearchInput(input)
	if err != nil {
		return ContractSearchPage{}, err
	}
	values, err := s.repository.Search(ctx, query)
	if err != nil {
		return ContractSearchPage{}, fmt.Errorf("search contracts: %w", err)
	}
	page := ContractSearchPage{
		Contracts: values, Keyword: query.Keyword, Status: query.Status, Currency: query.Currency,
	}
	if len(page.Contracts) <= contractSearchPageSize {
		return page, nil
	}
	page.Contracts = page.Contracts[:contractSearchPageSize]
	last := page.Contracts[len(page.Contracts)-1]
	page.NextCursor, err = encodeContractSearchCursor(last, query)
	if err != nil {
		return ContractSearchPage{}, fmt.Errorf("encode contract search cursor: %w", err)
	}
	return page, nil
}

func normalizeContractSearchInput(input SearchContractsInput) (ContractSearchQuery, error) {
	keyword := strings.TrimSpace(input.Keyword)
	if len([]rune(keyword)) > 100 {
		return ContractSearchQuery{}, &ValidationError{Field: "q", Message: "检索关键词不能超过 100 个字符"}
	}
	status := Status(strings.ToLower(strings.TrimSpace(input.Status)))
	switch status {
	case "", StatusDraft, StatusActive, StatusTerminated, StatusClosed, StatusCancelled:
	default:
		return ContractSearchQuery{}, &ValidationError{Field: "status", Message: "合同状态筛选无效"}
	}
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if currency != "" && !currencyPattern.MatchString(currency) {
		return ContractSearchQuery{}, &ValidationError{Field: "currency", Message: "币种筛选必须是三个英文字母"}
	}
	query := ContractSearchQuery{
		Keyword: keyword, Status: status, Currency: currency, Limit: contractSearchPageSize + 1,
	}
	if strings.TrimSpace(input.Cursor) == "" {
		return query, nil
	}
	if len(input.Cursor) > 1024 {
		return ContractSearchQuery{}, invalidContractSearchCursor()
	}
	cursor, err := decodeContractSearchCursor(input.Cursor)
	if err != nil || cursor.Keyword != keyword || cursor.Status != string(status) || cursor.Currency != currency {
		return ContractSearchQuery{}, invalidContractSearchCursor()
	}
	createdAt, err := time.Parse(time.RFC3339Nano, cursor.CreatedAt)
	if err != nil || !uuidPattern.MatchString(cursor.ID) {
		return ContractSearchQuery{}, invalidContractSearchCursor()
	}
	query.AfterCreatedAt = createdAt.UTC()
	query.AfterID = strings.ToLower(cursor.ID)
	return query, nil
}

func encodeContractSearchCursor(value Contract, query ContractSearchQuery) (string, error) {
	payload, err := json.Marshal(contractSearchCursor{
		Version: 1, CreatedAt: value.CreatedAt.UTC().Format(time.RFC3339Nano), ID: value.ID,
		Keyword: query.Keyword, Status: string(query.Status), Currency: query.Currency,
	})
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeContractSearchCursor(value string) (contractSearchCursor, error) {
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(value))
	if err != nil {
		return contractSearchCursor{}, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.DisallowUnknownFields()
	var cursor contractSearchCursor
	if err := decoder.Decode(&cursor); err != nil {
		return contractSearchCursor{}, err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return contractSearchCursor{}, fmt.Errorf("cursor has trailing data")
	}
	if cursor.Version != 1 {
		return contractSearchCursor{}, fmt.Errorf("unsupported cursor version")
	}
	return cursor, nil
}

func invalidContractSearchCursor() *ValidationError {
	return &ValidationError{Field: "cursor", Message: "分页游标无效，请从第一页重新查询"}
}
