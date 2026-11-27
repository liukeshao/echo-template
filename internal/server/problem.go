package server

import (
	"context"

	"github.com/samber/oops"
	"golang.org/x/text/language"

	"github.com/liukeshao/echo-template/internal/i18n"
	"github.com/liukeshao/echo-template/pkg/httperr"
	"github.com/liukeshao/echo-template/pkg/logging"
)

// NewProblemEncoder 装配应用级 RFC 9457 错误编码器（RequestID + oops Metadata + i18n）。
func NewProblemEncoder(tr i18n.Translator) httperr.Encoder {
	opts := []httperr.EncoderOption{
		httperr.WithRequestID(logging.RequestID),
		httperr.WithMetadataExtractor(extractOopsMetadata),
	}
	if tr != nil {
		opts = append(opts, httperr.WithTranslator(func(ctx context.Context, key string, vars map[string]string) (string, string, bool) {
			loc := i18n.Locale(ctx, tr.Default())
			msg, used, ok := tr.T(loc, key, vars)
			if !ok {
				return "", "", false
			}
			if used == language.Und {
				return msg, "", true
			}
			return msg, used.String(), true
		}))
	}
	return httperr.NewEncoder(opts...)
}

func extractOopsMetadata(err error) (httperr.Metadata, bool) {
	oe, ok := oops.AsOops(err)
	if !ok {
		return nil, false
	}
	m, ok := oe.Context()[httperr.MetaKey].(httperr.Metadata)
	return m, ok
}
