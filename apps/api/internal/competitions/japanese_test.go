package competitions

import "testing"

func TestLocalizeMeetName(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"World Athletics Relays", "世界リレー"},
		{"World Athletics Indoor Championships", "世界室内陸上"},
		{"World Athletics U20 Championships", "U20世界陸上"},
		{"World Athletics Road Running Championships", "世界ロードランニング"},
		{"World Athletics Championships, Beijing 2027", "世界陸上・北京"},
		{"World Athletics Ultimate Championship", "世界アルティメット選手権"},
		{"Caixa World Athletics Race Walking Team Championships Brasilia 26", "世界競歩チーム選手権・ブラジリア"},
		{"Wanda Diamond League Shanghai", "ダイヤモンドリーグ・上海"},
		{"Wanda Diamond League Xiamen", "ダイヤモンドリーグ・アモイ"},
		{"Wanda Diamond League Final", "ダイヤモンドリーグ・ファイナル"},
		{"World Athletics Continental Tour Gold", "コンチネンタルツアー・ゴールド"},
		{"World Athletics Continental Tour Gold in Xiamen", "コンチネンタルツアー・ゴールド・アモイ"},
		{"Los Angeles 2028 Olympic Games", "オリンピック・ロサンゼルス"},
		{"Allianz Memorial van Damme", "メモリアル・ヴァンダム"},
		{"Novuna London Athletics Meet", "ダイヤモンドリーグ・ロンドン"},
		{"Athletissima Lausanne", "アスレティッシマ"},
		{"Olso Bislett Games", "ビスレットゲームズ"},
		{"Oslo Bislett Games", "ビスレットゲームズ"},
		{"Meeting International Mohammed VI d'Athletisme de Rabat", "モハメド6世記念"},
		{"Meeting International d’Athlétisme Herculis EBS", "ヘラクレス"},
		{"Golden Gala Pietro Mennea", "ゴールデンガラ"},
		{"BAUHAUS-Galan", "バウハウスガラン"},
		{"Doha Meeting", "ダイヤモンドリーグ・ドーハ"},
		{"Silesia Kamila Skolimowska Memorial", "カミラスコリモフスカ記念"},
		{"Prefontaine Classic", "プレフォンテーンクラシック"},
		{"Bank of America Chicago Marathon", "シカゴマラソン"},
		{"TCS London Marathon", "ロンドンマラソン"},
		{"Boston Marathon", "ボストンマラソン"},
		{"BMW Berlin Marathon", "ベルリンマラソン"},
		{"TCS Amsterdam Marathon", "アムステルダムマラソン"},
		{"TCS New York City Marathon", "ニューヨークシティマラソン"},
		{"TCS Sydney Marathon Presented by ASICS", "シドニーマラソン"},
		{"Maratón Valencia Trinidad Alfonso Zurich", "バレンシアマラソン"},
		{"Shanghai Marathon", "上海マラソン"},
	}
	for _, tc := range cases {
		if got := localizeMeetName(tc.name); got != tc.want {
			t.Errorf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

func TestLocalizeLocation(t *testing.T) {
	cases := []struct {
		location string
		want     string
	}{
		{"China Textile City Sports Centre, Shaoxing/Keqiao (CHN)", "紹興/柯橋（中国）"},
		{"Shanghai Stadium, Shanghai (CHN)", "上海（中国）"},
		{"Egret Stadium, Xiamen (CHN)", "アモイ（中国）"},
		{"Hayward Field, Eugene, OR (USA)", "ユージーン（アメリカ）"},
		{"New York, NY (USA)", "ニューヨーク（アメリカ）"},
		{"Boston, MA (USA)", "ボストン（アメリカ）"},
		{"København (DEN)", "コペンハーゲン（デンマーク）"},
		{"Kalinga Indoor Stadium, Bhubaneshwar (IND)", "ブバネーシュワル（インド）"},
		{"Yangzhou (CHN)", "揚州（中国）"},
		{"Brasilia (BRA)", "ブラジリア（ブラジル）"},
		{"Stadion Śląski, Chorzów (POL)", "ホジュフ（ポーランド）"},
		{"Stade Charléty, Paris (FRA)", "パリ（フランス）"},
		{"Stadio Olimpico, Roma (ITA)", "ローマ（イタリア）"},
		{"Stade Louis II, Monaco (MON)", "モナコ"},
		{"Nemzeti Atlétikai Központ, Budapest (HUN)", "ブダペスト（ハンガリー）"},
		{"Letzigrund, Zürich (SUI)", "チューリッヒ（スイス）"},
		{"Mystery Track (USA)", "Mystery Track（アメリカ）"},
		{"Somewhere (XYZ)", "Somewhere (XYZ)"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := localizeLocation(tc.location); got != tc.want {
			t.Errorf("%s: got %q want %q", tc.location, got, tc.want)
		}
	}
}
