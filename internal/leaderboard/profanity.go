package leaderboard

import "strings"

// 公司名稱髒話過濾。獨立檔案維護，前端有獨立清單做第一道防線。
// 後端做二次校驗以防客戶端被繞過。

var profanityWords = []string{
	// 中文
	"幹", "靠北", "靠杯", "操你", "幹你", "草泥馬", "雞掰", "雞八", "機掰",
	"王八蛋", "畜生", "智障", "白癡", "低能", "北七", "婊子", "妓女", "賤人",
	"狗娘", "婊", "幹爆", "幹妳", "操妳",
	// 英文
	"fuck", "shit", "cunt", "bitch", "asshole", "bastard", "dick", "pussy",
	"whore", "slut", "nigger", "faggot",
}

// containsProfanity returns true if name contains any profanity word (case-insensitive).
func containsProfanity(name string) bool {
	lower := strings.ToLower(name)
	for _, w := range profanityWords {
		if strings.Contains(lower, strings.ToLower(w)) {
			return true
		}
	}
	return false
}
