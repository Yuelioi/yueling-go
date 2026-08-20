package imageinput

import (
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/Yuelioi/yueling-go/bot"
	"github.com/Yuelioi/yueling-go/services/httpclient"
)

type inputContextStub struct {
	urls     []string
	message  bot.Message
	userID   int64
	nickname string
}

func (c inputContextStub) CollectImageURLs() []string { return append([]string(nil), c.urls...) }
func (c inputContextStub) Message() bot.Message       { return c.message }
func (c inputContextStub) UserID() int64              { return c.userID }
func (c inputContextStub) Nickname() string           { return c.nickname }

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func stubPublicDownloads(t *testing.T) {
	t.Helper()
	previous := httpclient.Public
	httpclient.Public = &httpclient.Client{Client: &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(request.URL.String())),
			Request:    request,
		}, nil
	})}}
	t.Cleanup(func() { httpclient.Public = previous })
}

func TestResolveInputsUsesOneSourcePolicy(t *testing.T) {
	stubPublicDownloads(t)

	const (
		senderID = int64(10001)
		targetID = int64(20002)
	)
	tests := []struct {
		name string
		ctx  inputContextStub
		want []string
	}{
		{
			name: "current or replied image wins over mention",
			ctx: inputContextStub{
				urls:     []string{"https://images.example/context.png"},
				message:  bot.Msg().At(targetID).Build(),
				userID:   senderID,
				nickname: "发送者",
			},
			want: []string{"https://images.example/context.png"},
		},
		{
			name: "mention supplies target avatar",
			ctx: inputContextStub{
				message:  bot.Msg().At(targetID).Build(),
				userID:   senderID,
				nickname: "发送者",
			},
			want: []string{QQAvatarURL(targetID)},
		},
		{
			name: "sender avatar is the default",
			ctx: inputContextStub{
				userID:   senderID,
				nickname: "发送者",
			},
			want: []string{QQAvatarURL(senderID)},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			items, err := ResolveInputs(test.ctx, 1, 1)
			if err != nil {
				t.Fatal(err)
			}
			got := make([]string, len(items))
			for i := range items {
				got[i] = string(items[i].Data)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("resolved inputs = %q, want %q", got, test.want)
			}
		})
	}
}

func TestResolveInputsFillsMultipleRequiredSlots(t *testing.T) {
	stubPublicDownloads(t)

	const (
		senderID = int64(10001)
		targetID = int64(20002)
	)
	tests := []struct {
		name    string
		message bot.Message
		want    []string
	}{
		{
			name:    "one mention means sender then target",
			message: bot.Msg().At(targetID).Build(),
			want:    []string{QQAvatarURL(senderID), QQAvatarURL(targetID)},
		},
		{
			name: "no explicit input repeats sender avatar",
			want: []string{QQAvatarURL(senderID), QQAvatarURL(senderID)},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			items, err := ResolveInputs(inputContextStub{
				message:  test.message,
				userID:   senderID,
				nickname: "发送者",
			}, 2, 2)
			if err != nil {
				t.Fatal(err)
			}
			got := make([]string, len(items))
			for i := range items {
				got[i] = string(items[i].Data)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("resolved inputs = %q, want %q", got, test.want)
			}
		})
	}
}

func TestResolveInputsSkipsImageLookupForTextOnlyTemplates(t *testing.T) {
	items, err := ResolveInputs(inputContextStub{}, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("resolved %d inputs, want none", len(items))
	}
}
