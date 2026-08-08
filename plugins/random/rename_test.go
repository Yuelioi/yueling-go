package random

import (
	"errors"
	"testing"
)

type renameContextSpy struct {
	groupID int64
	userID  int64
	card    string
	setErr  error
	replies []string
}

func (c *renameContextSpy) GroupID() int64 { return c.groupID }
func (c *renameContextSpy) UserID() int64  { return c.userID }

func (c *renameContextSpy) SetGroupCard(groupID, userID int64, card string) error {
	if groupID != c.groupID || userID != c.userID {
		return errors.New("unexpected rename target")
	}
	c.card = card
	return c.setErr
}

func (c *renameContextSpy) Reply(text string) error {
	c.replies = append(c.replies, text)
	return nil
}

func TestHandleRenameChangesCardWithoutReply(t *testing.T) {
	ctx := &renameContextSpy{groupID: 100, userID: 200}

	if err := handleRename(ctx, nil); err != nil {
		t.Fatalf("handleRename() error = %v", err)
	}
	if ctx.card == "" {
		t.Fatal("handleRename() did not set a group card")
	}
	if len(ctx.replies) != 0 {
		t.Fatalf("handleRename() sent replies = %q, want none", ctx.replies)
	}
}

func TestHandleRenameFailureDoesNotReply(t *testing.T) {
	wantErr := errors.New("permission denied")
	ctx := &renameContextSpy{groupID: 100, userID: 200, setErr: wantErr}

	err := handleRename(ctx, nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("handleRename() error = %v, want %v", err, wantErr)
	}
	if len(ctx.replies) != 0 {
		t.Fatalf("handleRename() sent replies = %q, want none", ctx.replies)
	}
}

func TestNameDictionariesAreComplete(t *testing.T) {
	if len(nameDictionaries) < 4 {
		t.Fatalf("len(nameDictionaries) = %d, want at least 4", len(nameDictionaries))
	}
	for i, dictionary := range nameDictionaries {
		if len(dictionary.first) == 0 || len(dictionary.second) == 0 || len(dictionary.third) == 0 {
			t.Fatalf("nameDictionaries[%d] contains an empty word list", i)
		}
	}
}
