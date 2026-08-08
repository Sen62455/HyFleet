package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

const maxHysteriaAuthBodyBytes = 4096

type hysteriaAuthRequest struct {
	Address string `json:"addr"`
	Auth    string `json:"auth"`
	TX      int64  `json:"tx"`
}

type hysteriaAuthResponse struct {
	OK bool   `json:"ok"`
	ID string `json:"id,omitempty"`
}

func newHysteriaAuthHandler(cache *AuthCache, path string, now func() time.Time) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(path, func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "application/json; charset=utf-8")
		response.Header().Set("Cache-Control", "no-store")
		if request.Method != http.MethodPost {
			response.Header().Set("Allow", http.MethodPost)
			response.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		request.Body = http.MaxBytesReader(response, request.Body, maxHysteriaAuthBodyBytes)
		decoder := json.NewDecoder(request.Body)
		var input hysteriaAuthRequest
		if err := decoder.Decode(&input); err != nil {
			writeHysteriaAuthResponse(response, "", false)
			return
		}
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			writeHysteriaAuthResponse(response, "", false)
			return
		}
		if input.Auth == "" || len(input.Auth) > 512 || len(input.Address) > 256 || input.TX < 0 {
			writeHysteriaAuthResponse(response, "", false)
			return
		}
		userID, accepted := cache.Authenticate(input.Auth, now())
		writeHysteriaAuthResponse(response, userID, accepted)
	})
	return mux
}

func writeHysteriaAuthResponse(response http.ResponseWriter, userID string, accepted bool) {
	response.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(response).Encode(hysteriaAuthResponse{OK: accepted, ID: userID})
}

func (agent *Agent) startNativeAuthServer(ctx context.Context) (<-chan error, func(), error) {
	listener, err := net.Listen("tcp", agent.config.AuthListen)
	if err != nil {
		return nil, nil, fmt.Errorf("listen for native Hysteria2 authentication: %w", err)
	}
	server := &http.Server{
		Handler:           newHysteriaAuthHandler(agent.authCache, agent.config.AuthPath, time.Now),
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      5 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    8 * 1024,
	}
	errorsChannel := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errorsChannel <- err
		}
		close(errorsChannel)
	}()
	go func() {
		<-ctx.Done()
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownContext)
	}()
	stop := func() {
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownContext)
	}
	agent.logger.Info("native Hysteria2 auth listener ready", "address", agent.config.AuthListen)
	return errorsChannel, stop, nil
}
