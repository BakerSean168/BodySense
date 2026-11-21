package main

import (
	"log/slog"

	"github.com/gin-gonic/gin"
)

// sanitizeHTTPRequestLogRecord keeps gin-contrib/slog's mature request lifecycle
// handling while removing fields that are not appropriate for a health product's
// operational logs. Route/status/latency/request-id are enough for correlation;
// raw query strings, client IPs and referrers are intentionally excluded.
func sanitizeHTTPRequestLogRecord(_ *gin.Context, record *slog.Record) *slog.Record {
	clean := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		switch attr.Key {
		case "query", "ip", "referer":
			return true
		default:
			clean.AddAttrs(attr)
			return true
		}
	})
	return &clean
}
