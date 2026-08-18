package httpapi

import (
	"fmt"
	"net/http"

	"github.com/diegoochoa/calculator-sezzle-api/internal/calc"
	"github.com/diegoochoa/calculator-sezzle-api/internal/httpx"
)

// maxPrecision is the most significant digits a client may ask for; beyond 15
// a float64 is inventing digits it does not have.
const maxPrecision = 15

type calculateRequest struct {
	Expression string `json:"expression"`
	Precision  int    `json:"precision"`
}

type calculateResponse struct {
	Result     float64 `json:"result"`
	Formatted  string  `json:"formatted"`
	Expression string  `json:"expression"`
}

type batchRequest struct {
	Expressions []string `json:"expressions"`
	Precision   int      `json:"precision"`
}

type batchItem struct {
	Expression string           `json:"expression"`
	Result     *float64         `json:"result,omitempty"`
	Formatted  string           `json:"formatted,omitempty"`
	Error      *httpx.ErrorBody `json:"error,omitempty"`
}

type batchResponse struct {
	Results []batchItem `json:"results"`
}

type validateRequest struct {
	Expression string `json:"expression"`
}

type validateResponse struct {
	Valid      bool             `json:"valid"`
	Expression string           `json:"expression"`
	Error      *httpx.ErrorBody `json:"error,omitempty"`
}

// resolveOptions validates the precision shared by the calculation endpoints.
func (s *Server) resolveOptions(w http.ResponseWriter, r *http.Request, precision int) (calc.Options, bool) {
	options := s.calcOptions
	if precision != 0 {
		if precision < 1 || precision > maxPrecision {
			httpx.WriteError(w, r, http.StatusBadRequest, httpx.CodeInvalidRequest,
				fmt.Sprintf("precision must be between 1 and %d", maxPrecision))
			return calc.Options{}, false
		}
		options.Precision = precision
	}
	return options, true
}

// handleCalculate evaluates a single expression.
func (s *Server) handleCalculate(w http.ResponseWriter, r *http.Request) {
	var request calculateRequest
	if !decodeJSON(w, r, &request) {
		return
	}

	options, ok := s.resolveOptions(w, r, request.Precision)
	if !ok {
		return
	}

	result, err := calc.Calculate(request.Expression, options)
	if err != nil {
		writeEngineError(w, r, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, calculateResponse{
		Result:     result.Value,
		Formatted:  result.Formatted,
		Expression: request.Expression,
	})
}

// handleBatch evaluates many expressions in one round trip.
//
// The response is a 200 even when individual expressions fail: a partial
// failure is the expected outcome here, and collapsing it into a single status
// would throw away the results that did succeed.
func (s *Server) handleBatch(w http.ResponseWriter, r *http.Request) {
	var request batchRequest
	if !decodeJSON(w, r, &request) {
		return
	}

	switch {
	case len(request.Expressions) == 0:
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.CodeInvalidRequest,
			"expressions must contain at least one entry")
		return
	case len(request.Expressions) > s.maxBatch:
		httpx.WriteError(w, r, http.StatusBadRequest, httpx.CodeInvalidRequest,
			fmt.Sprintf("expressions must contain at most %d entries", s.maxBatch))
		return
	}

	options, ok := s.resolveOptions(w, r, request.Precision)
	if !ok {
		return
	}

	results := make([]batchItem, 0, len(request.Expressions))
	for _, expression := range request.Expressions {
		item := batchItem{Expression: expression}

		if result, err := calc.Calculate(expression, options); err != nil {
			item.Error = errorBody(err)
		} else {
			value := result.Value
			item.Result = &value
			item.Formatted = result.Formatted
		}

		results = append(results, item)
	}

	httpx.WriteJSON(w, http.StatusOK, batchResponse{Results: results})
}

// handleValidate parses without evaluating.
//
// An invalid expression is a 200 with valid:false, not a 422: the client asked
// whether the text parses, and "no" is a successful answer to that question.
// This is what lets the UI check syntax on every keystroke without a division
// by zero or a slow computation.
func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	var request validateRequest
	if !decodeJSON(w, r, &request) {
		return
	}

	response := validateResponse{Valid: true, Expression: request.Expression}
	if err := calc.Validate(request.Expression, s.calcOptions); err != nil {
		response.Valid = false
		response.Error = errorBody(err)
	}

	httpx.WriteJSON(w, http.StatusOK, response)
}

// handleFunctions describes the grammar so a client can build its keypad and
// syntax hints from the server's real capability.
func (s *Server) handleFunctions(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, calc.NewCatalog(s.calcOptions))
}
