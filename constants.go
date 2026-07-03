package youtubedl

var Clients = map[string]YoutubeClient{
	"ANDROID": {
		Name:       "ANDROID",
		Version:    "20.10.38",
		UserAgent:  "com.google.android.youtube/20.10.38 (Linux; U; Android 11) gzip",
		SDKVersion: 30,
	},
	"IOS": {
		Name:        "IOS",
		Version:     "21.02.3",
		UserAgent:   "com.google.ios.youtube/21.02.3 (iPhone16,2; U; CPU iOS 18_3_2 like Mac OS X;)",
		DeviceModel: "iPhone16,2",
	},
	"TV_EMBEDDED": {
		Name:    "TVHTML5_SIMPLY_EMBEDDED_PLAYER",
		Version: "2.0",
	},
	"WEB": {
		Name:       "WEB",
		Version:    "2.20240726.00.00",
		APIKey:     "AIzaSyAO_FJ2SlqU8Q4STEHLGCilw_Y9_11qcW8",
		APIVersion: "v1",
	},
	"ANDROID_VR": {
		Name:        "ANDROID_VR",
		Version:     "1.65.10",
		UserAgent:   "com.google.android.apps.youtube.vr.oculus/1.65.10 (Linux; U; Android 12L; eureka-user Build/SQ3A.220605.009.A1) gzip",
		DeviceModel: "Quest 3",
		SDKVersion:  32,
	},
}

var ClientNameIDs = map[string]string{
	"WEB":                            "1",
	"ANDROID":                        "3",
	"IOS":                            "5",
	"TVHTML5_SIMPLY_EMBEDDED_PLAYER": "85",
	"ANDROID_VR":                     "28",
}

var URLs = struct {
	YTBase string
}{
	YTBase: "https://www.youtube.com",
}
