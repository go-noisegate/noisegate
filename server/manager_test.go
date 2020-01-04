package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleNextTaskSet(t *testing.T) {
	manager := NewManager("")
	req := httptest.NewRequest(http.MethodGet, nextTaskSetPath, nil)
	resp := httptest.NewRecorder()
	manager.handleNextTaskSet(resp, req)
}
