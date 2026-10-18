// Package jst は日本時間のタイムゾーンを1か所にまとめる。
package jst

import "time"

// Zone は日本時間。サーバーに tzdata がなくても動くよう、固定の +09:00 にする。
var Zone = time.FixedZone("Asia/Tokyo", 9*60*60)
