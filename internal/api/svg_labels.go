package api

// The snapshot SVG is a file the user downloads and shares, so it follows the
// dashboard language instead of staying English forever. The table is small on
// purpose: only the labels this image draws, with English as the fallback for
// an unknown or missing locale.

type svgLabels struct {
	Title      string
	Requests   string
	Tokens     string
	Spent      string
	Latency    string
	Prompt     string
	Completion string
	CacheRead  string
	TTFT       string
	Shadow     string
	AllTime    string
}

var svgLabelsEN = svgLabels{
	Title: "odotoken usage snapshot", Requests: "REQUESTS", Tokens: "TOTAL TOKENS",
	Spent: "TOTAL SPENT", Latency: "AVG LATENCY", Prompt: "Prompt",
	Completion: "Completion", CacheRead: "Cache read", TTFT: "Avg TTFT",
	Shadow: "Shadow cost", AllTime: "all time",
}

var svgLabelsByLocale = map[string]svgLabels{
	"pt-BR": {
		Title: "instantâneo de uso do odotoken", Requests: "REQUISIÇÕES", Tokens: "TOKENS TOTAIS",
		Spent: "TOTAL GASTO", Latency: "LATÊNCIA MÉDIA", Prompt: "Prompt",
		Completion: "Conclusão", CacheRead: "Leitura de cache", TTFT: "TTFT médio",
		Shadow: "Custo sombra", AllTime: "todo o período",
	},
	"es": {
		Title: "instantánea de uso de odotoken", Requests: "SOLICITUDES", Tokens: "TOKENS TOTALES",
		Spent: "TOTAL GASTADO", Latency: "LATENCIA MEDIA", Prompt: "Prompt",
		Completion: "Finalización", CacheRead: "Lectura de caché", TTFT: "TTFT medio",
		Shadow: "Costo sombra", AllTime: "todo el período",
	},
	"fr": {
		Title: "instantané d'usage odotoken", Requests: "REQUÊTES", Tokens: "TOKENS TOTAUX",
		Spent: "TOTAL DÉPENSÉ", Latency: "LATENCE MOY.", Prompt: "Invite",
		Completion: "Réponse", CacheRead: "Lecture cache", TTFT: "TTFT moyen",
		Shadow: "Coût fantôme", AllTime: "depuis le début",
	},
	"it": {
		Title: "istantanea di utilizzo odotoken", Requests: "RICHIESTE", Tokens: "TOKEN TOTALI",
		Spent: "TOTALE SPESO", Latency: "LATENZA MEDIA", Prompt: "Prompt",
		Completion: "Completamento", CacheRead: "Lettura cache", TTFT: "TTFT medio",
		Shadow: "Costo ombra", AllTime: "dall'inizio",
	},
	"de": {
		Title: "odotoken Nutzungsübersicht", Requests: "ANFRAGEN", Tokens: "TOKEN GESAMT",
		Spent: "GESAMTAUSGABEN", Latency: "Ø LATENZ", Prompt: "Prompt",
		Completion: "Antwort", CacheRead: "Cache-Lesen", TTFT: "Ø TTFT",
		Shadow: "Schattenkosten", AllTime: "gesamter Zeitraum",
	},
	"ru": {
		Title: "снимок использования odotoken", Requests: "ЗАПРОСЫ", Tokens: "ВСЕГО ТОКЕНОВ",
		Spent: "ВСЕГО ПОТРАЧЕНО", Latency: "СР. ЗАДЕРЖКА", Prompt: "Промпт",
		Completion: "Ответ", CacheRead: "Чтение кэша", TTFT: "Ср. TTFT",
		Shadow: "Теневая стоимость", AllTime: "за всё время",
	},
	"uk": {
		Title: "знімок використання odotoken", Requests: "ЗАПИТИ", Tokens: "УСЬОГО ТОКЕНІВ",
		Spent: "УСЬОГО ВИТРАЧЕНО", Latency: "СЕР. ЗАТРИМКА", Prompt: "Промпт",
		Completion: "Відповідь", CacheRead: "Читання кешу", TTFT: "Сер. TTFT",
		Shadow: "Тіньова вартість", AllTime: "за весь час",
	},
	"pl": {
		Title: "zrzut użycia odotoken", Requests: "ŻĄDANIA", Tokens: "TOKENY RAZEM",
		Spent: "ŁĄCZNIE WYDANO", Latency: "ŚR. OPÓŹNIENIE", Prompt: "Prompt",
		Completion: "Uzupełnienie", CacheRead: "Odczyt cache", TTFT: "Śr. TTFT",
		Shadow: "Koszt cienia", AllTime: "cały okres",
	},
	"tr": {
		Title: "odotoken kullanım anlık görüntüsü", Requests: "İSTEKLER", Tokens: "TOPLAM TOKEN",
		Spent: "TOPLAM HARCAMA", Latency: "ORT. GECİKME", Prompt: "İstem",
		Completion: "Tamamlama", CacheRead: "Önbellek okuma", TTFT: "Ort. TTFT",
		Shadow: "Gölge maliyet", AllTime: "tüm zamanlar",
	},
	"hi": {
		Title: "odotoken उपयोग स्नैपशॉट", Requests: "अनुरोध", Tokens: "कुल टोकन",
		Spent: "कुल खर्च", Latency: "औसत विलंबता", Prompt: "प्रॉम्प्ट",
		Completion: "पूर्णता", CacheRead: "कैश पठन", TTFT: "औसत TTFT",
		Shadow: "छाया लागत", AllTime: "पूरी अवधि",
	},
	"id": {
		Title: "cuplikan penggunaan odotoken", Requests: "PERMINTAAN", Tokens: "TOTAL TOKEN",
		Spent: "TOTAL BIAYA", Latency: "RATA-RATA LATENSI", Prompt: "Prompt",
		Completion: "Penyelesaian", CacheRead: "Baca cache", TTFT: "Rata-rata TTFT",
		Shadow: "Biaya bayangan", AllTime: "sepanjang waktu",
	},
	"vi": {
		Title: "ảnh chụp mức dùng odotoken", Requests: "YÊU CẦU", Tokens: "TỔNG TOKEN",
		Spent: "TỔNG CHI", Latency: "ĐỘ TRỄ TB", Prompt: "Prompt",
		Completion: "Hoàn thành", CacheRead: "Đọc cache", TTFT: "TTFT TB",
		Shadow: "Chi phí bóng", AllTime: "toàn bộ thời gian",
	},
	"th": {
		Title: "ภาพรวมการใช้งาน odotoken", Requests: "คำขอ", Tokens: "โทเคนทั้งหมด",
		Spent: "ใช้จ่ายทั้งหมด", Latency: "เวลาแฝงเฉลี่ย", Prompt: "พรอมป์ต์",
		Completion: "ผลลัพธ์", CacheRead: "อ่านแคช", TTFT: "TTFT เฉลี่ย",
		Shadow: "ต้นทุนเงา", AllTime: "ทั้งหมด",
	},
	"ar": {
		Title: "لقطة استخدام odotoken", Requests: "الطلبات", Tokens: "إجمالي الرموز",
		Spent: "إجمالي المصروف", Latency: "متوسط التأخير", Prompt: "المطالبة",
		Completion: "الإكمال", CacheRead: "قراءة الذاكرة", TTFT: "متوسط TTFT",
		Shadow: "التكلفة الظلية", AllTime: "كل الفترات",
	},
	"fa": {
		Title: "عکس لحظه‌ای مصرف odotoken", Requests: "درخواست‌ها", Tokens: "کل توکن‌ها",
		Spent: "کل هزینه", Latency: "میانگین تأخیر", Prompt: "پرامپت",
		Completion: "تکمیل", CacheRead: "خواندن کش", TTFT: "میانگین TTFT",
		Shadow: "هزینه سایه‌ای", AllTime: "کل بازه",
	},
	"zh-CN": {
		Title: "odotoken 用量快照", Requests: "请求数", Tokens: "总令牌数",
		Spent: "总花费", Latency: "平均延迟", Prompt: "提示",
		Completion: "补全", CacheRead: "缓存读取", TTFT: "平均 TTFT",
		Shadow: "影子成本", AllTime: "全部时间",
	},
	"zh-TW": {
		Title: "odotoken 用量快照", Requests: "請求數", Tokens: "總權杖數",
		Spent: "總花費", Latency: "平均延遲", Prompt: "提示",
		Completion: "補全", CacheRead: "快取讀取", TTFT: "平均 TTFT",
		Shadow: "影子成本", AllTime: "全部時間",
	},
	"ja": {
		Title: "odotoken 使用状況スナップショット", Requests: "リクエスト", Tokens: "総トークン",
		Spent: "総支出", Latency: "平均レイテンシ", Prompt: "プロンプト",
		Completion: "補完", CacheRead: "キャッシュ読取", TTFT: "平均 TTFT",
		Shadow: "シャドウ費用", AllTime: "全期間",
	},
	"ko": {
		Title: "odotoken 사용 현황 스냅숏", Requests: "요청", Tokens: "총 토큰",
		Spent: "총 지출", Latency: "평균 지연", Prompt: "프롬프트",
		Completion: "완성", CacheRead: "캐시 읽기", TTFT: "평균 TTFT",
		Shadow: "그림자 비용", AllTime: "전체 기간",
	},
}

// svgLabelsFor resolves a requested language tag to the labels this image
// draws. An unknown locale falls back to English rather than drawing a raw key.
func svgLabelsFor(lang string) svgLabels {
	if labels, ok := svgLabelsByLocale[lang]; ok {
		return labels
	}
	// `pt`, `pt-BR`, `pt-PT` all share the one Portuguese catalog we ship.
	for _, prefix := range []string{"pt", "zh-Hant", "zh-TW", "zh"} {
		if len(lang) >= len(prefix) && lang[:len(prefix)] == prefix {
			if labels, ok := svgLabelsByLocale[map[string]string{
				"pt": "pt-BR", "zh": "zh-CN", "zh-Hant": "zh-TW", "zh-TW": "zh-TW",
			}[prefix]]; ok {
				return labels
			}
		}
	}
	return svgLabelsEN
}
