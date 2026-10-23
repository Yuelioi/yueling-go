package catalog

const (
	PluginBan              = 1
	PluginJoinReview       = 3
	PluginMemberBackup     = 4
	PluginRules            = 5
	PluginFiles            = 7
	PluginUserProfile      = 8
	PluginCheckIn          = 9
	PluginJW3              = 11
	PluginClockIn          = 12
	PluginReminder         = 13
	PluginRandomMember     = 14
	PluginRandomRename     = 15
	PluginRoll             = 16
	PluginEmoticon         = 17
	PluginRandomImage      = 18
	PluginFortune          = 20
	PluginHot              = 21
	PluginSleep            = 23
	PluginPoke             = 24
	PluginTraceMoe         = 25
	PluginLinkAnalysis     = 28
	PluginAIAssistant      = 29
	PluginHelp             = 30
	PluginSystemTools      = 31
	PluginUploadAssets     = 32
	PluginMemes            = 33
	PluginPack             = 34
	PluginZSSM             = 35 // Reserved so existing disabled-plugin settings remain readable.
	PluginEssence          = 36
	PluginDailyDigest      = 37
	PluginFeedSubscription = 38
	PluginGroupKnowledge   = 39
	PluginChatStats        = 40
	PluginGameDeals        = 41
	PluginImageOps         = 42
	PluginAvatarMeme       = 43
	PluginBilibili         = 44
	PluginWuxia            = 45
)

func Known(id int) bool {
	switch id {
	case PluginBan, PluginJoinReview, PluginMemberBackup, PluginRules, PluginFiles, PluginUserProfile,
		PluginCheckIn, PluginJW3, PluginClockIn, PluginReminder, PluginRandomMember, PluginRandomRename,
		PluginRoll, PluginEmoticon, PluginRandomImage, PluginFortune, PluginHot, PluginSleep, PluginPoke,
		PluginTraceMoe, PluginLinkAnalysis, PluginAIAssistant, PluginHelp, PluginSystemTools, PluginUploadAssets,
		PluginMemes, PluginPack, PluginZSSM, PluginEssence, PluginDailyDigest, PluginFeedSubscription,
		PluginGroupKnowledge, PluginChatStats, PluginGameDeals, PluginImageOps, PluginAvatarMeme, PluginBilibili, PluginWuxia:
		return true
	}
	return false
}
