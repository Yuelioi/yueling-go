package bot

import "fmt"

// QQAvatarURL resolves a QQ identity to the same image source used by avatar memes.
func QQAvatarURL(userID int64) string {
	if userID <= 0 {
		return ""
	}
	return fmt.Sprintf("https://q.qlogo.cn/headimg_dl?dst_uin=%d&spec=640&img_type=jpg", userID)
}
