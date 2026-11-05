package govfiling

import (
	"context"

	"github.com/halaostory/halaos/internal/ai/provider"
)

type fakeAI struct{ reply, prompt string }

func (f *fakeAI) Name() string { return "fake" }
func (f *fakeAI) Generate(ctx context.Context, r provider.Request) (*provider.Response, error) {
	for _, m := range r.Messages {
		f.prompt += m.Content
	}
	return &provider.Response{Content: f.reply}, nil
}
func (f *fakeAI) Stream(ctx context.Context, r provider.Request, cb func(provider.StreamChunk)) (*provider.Response, error) {
	return f.Generate(ctx, r)
}
func (f *fakeAI) Health(ctx context.Context) error { return nil }
