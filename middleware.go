package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const (
	X_REQUEST_ID    = "X-Request-ID"
	requestInfoMsg  = "request_information"
	responseInfoMsg = "response_information"
	apiSummary      = "api_summary"
)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		var xid = c.Request.Header.Get(X_REQUEST_ID)
		if xid == "" {
			xid = uuid.New().String()
		}
		c.Set(X_REQUEST_ID, xid)
		c.Request.Header.Set(X_REQUEST_ID, xid)
		c.Next()
	}
}

func Logger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.FullPath(), "/liveness") || strings.HasPrefix(c.FullPath(), "/readiness") {
			c.Next()
			return
		}

		start := time.Now()
		c.Next()
		path := c.FullPath()
		method := c.Request.Method
		status := c.Writer.Status()
		logger.Info(fmt.Sprintf("%s: method=%s, path=%s, status=%d", apiSummary, method, path, status),
			zap.String("xid", getRequestID(c)),
			zap.String("method", method),
			zap.String("path_uri", path),
			zap.Int("status", c.Writer.Status()),
			zap.String("latency", time.Since(start).String()),
		)
	}
}

func RequestLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if strings.HasPrefix(c.FullPath(), "/liveness") || strings.HasPrefix(c.FullPath(), "/readiness") {
			c.Next()
			return
		}

		header, _ := json.Marshal(c.Request.Header)
		body, _ := io.ReadAll(c.Request.Body)
		zf := []zap.Field{
			zap.String("xid", getRequestID(c)),
			zap.String("method", c.Request.Method),
			zap.String("path_uri", c.FullPath()),
			zap.String("header", string(header)),
			zap.String("body", string(body)),
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))

		if logger.Level() == zapcore.InfoLevel {
			logger.Info(requestInfoMsg, zf[:3]...)
		} else {
			logger.Debug(requestInfoMsg, zf...)
		}

		c.Next()
	}
}

type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (r responseBodyWriter) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func ResponseLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if logger.Level() == zapcore.InfoLevel || strings.HasPrefix(c.FullPath(), "/liveness") || strings.HasPrefix(c.FullPath(), "/readiness") {
			c.Next()
			return
		}

		w := &responseBodyWriter{body: &bytes.Buffer{}, ResponseWriter: c.Writer}
		c.Writer = w
		c.Next()
		logger.Debug(responseInfoMsg,
			zap.String("xid", getRequestID(c)),
			zap.String("body", w.body.String()),
			zap.Int("status", w.Status()),
		)
	}
}

func getRequestID(c *gin.Context) string {
	return c.Request.Header.Get(X_REQUEST_ID)
}
