package bot

// Sender carries member info from event payloads and group member list responses.
type Sender struct {
	UserID       int64  `json:"user_id"`
	Nickname     string `json:"nickname"`
	Card         string `json:"card"`
	Role         string `json:"role"` // "owner" | "admin" | "member"
	Sex          string `json:"sex"`
	Age          int    `json:"age"`
	Title        string `json:"title"`
	LastSentTime int64  `json:"last_sent_time"`
}

// GroupMessageEvent is a message sent inside a QQ group.
type GroupMessageEvent struct {
	Time        int64   `json:"time"`
	SelfID      int64   `json:"self_id"`
	PostType    string  `json:"post_type"`
	MessageType string  `json:"message_type"`
	SubType     string  `json:"sub_type"` // "normal" | "anonymous" | "notice"
	MessageID   int32   `json:"message_id"`
	GroupID     int64   `json:"group_id"`
	UserID      int64   `json:"user_id"`
	Message     Message `json:"message"`
	RawMessage  string  `json:"raw_message"`
	Sender      Sender  `json:"sender"`
}

// PrivateMessageEvent is a direct message from a user.
type PrivateMessageEvent struct {
	Time        int64   `json:"time"`
	SelfID      int64   `json:"self_id"`
	PostType    string  `json:"post_type"`
	MessageType string  `json:"message_type"`
	SubType     string  `json:"sub_type"`
	MessageID   int32   `json:"message_id"`
	UserID      int64   `json:"user_id"`
	Message     Message `json:"message"`
	RawMessage  string  `json:"raw_message"`
	Sender      Sender  `json:"sender"`
}

// NoticeEvent covers group_increase, group_decrease, recall, poke, etc.
type NoticeEvent struct {
	Time       int64  `json:"time"`
	SelfID     int64  `json:"self_id"`
	PostType   string `json:"post_type"`
	NoticeType string `json:"notice_type"`
	GroupID    int64  `json:"group_id,omitempty"`
	UserID     int64  `json:"user_id,omitempty"`
	OperatorID int64  `json:"operator_id,omitempty"`
	TargetID   int64  `json:"target_id,omitempty"`
	MessageID  int32  `json:"message_id,omitempty"`
	SubType    string `json:"sub_type,omitempty"`
}

// RequestEvent covers friend requests and group join requests.
type RequestEvent struct {
	Time        int64  `json:"time"`
	SelfID      int64  `json:"self_id"`
	PostType    string `json:"post_type"`
	RequestType string `json:"request_type"` // "friend" | "group"
	UserID      int64  `json:"user_id"`
	GroupID     int64  `json:"group_id,omitempty"`
	SubType     string `json:"sub_type,omitempty"` // "add" | "invite"
	Comment     string `json:"comment"`
	Flag        string `json:"flag"`
}

// rawEvent is used only to peek post_type / echo before full parsing.
type rawEvent struct {
	PostType    string `json:"post_type"`
	MessageType string `json:"message_type"`
	NoticeType  string `json:"notice_type"`
	RequestType string `json:"request_type"`
	Echo        string `json:"echo"`
}
