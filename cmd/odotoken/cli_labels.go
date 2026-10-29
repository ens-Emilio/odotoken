package main

import (
	"os"
	"strings"
)

// Human CLI output follows the terminal language; machine output (`--json`)
// never does, because scripts parse it. Only the labels this CLI prints live
// here — prose the user reads, not identifiers like CLI names or paths.

type cliLabels struct {
	status     string
	files      string
	missing    string
	ready      string
	scanned    string
	new        string
	dupes      string
	cost       string
	total      string
	tokens     string
	unpriced   string
	importWord string
	dryRun     string
}

var cliLabelsEN = cliLabels{
	status: "STATUS", files: "FILES", missing: "missing", ready: "ready",
	scanned: "SCANNED", new: "new", dupes: "duplicates", cost: "COST",
	total: "Total", tokens: "tokens",
	unpriced:   "events had no known price and were recorded at $0.",
	importWord: "Import", dryRun: "Dry run: nothing was written.",
}

var cliLabelsByLocale = map[string]cliLabels{
	"pt-BR": {
		status: "ESTADO", files: "ARQUIVOS", missing: "ausente", ready: "pronto",
		scanned: "LIDOS", new: "novos", dupes: "duplicados", cost: "CUSTO",
		total: "Total", tokens: "tokens",
		unpriced:   "eventos não tinham preço conhecido e foram gravados a $0.",
		importWord: "Importação", dryRun: "Simulação: nada foi gravado.",
	},
	"es": {
		status: "ESTADO", files: "ARCHIVOS", missing: "ausente", ready: "listo",
		scanned: "LEÍDOS", new: "nuevos", dupes: "duplicados", cost: "COSTE",
		total: "Total", tokens: "tokens",
		unpriced:   "eventos sin precio conocido se registraron a $0.",
		importWord: "Importación", dryRun: "Simulación: no se escribió nada.",
	},
	"de": {
		status: "STATUS", files: "DATEIEN", missing: "fehlt", ready: "bereit",
		scanned: "GELESEN", new: "neu", dupes: "Duplikate", cost: "KOSTEN",
		total: "Gesamt", tokens: "Token",
		unpriced:   "Ereignisse ohne bekannten Preis wurden mit $0 erfasst.",
		importWord: "Import", dryRun: "Probelauf: Es wurde nichts geschrieben.",
	},
	"fr": {
		status: "ÉTAT", files: "FICHIERS", missing: "absent", ready: "prêt",
		scanned: "LUS", new: "nouveaux", dupes: "doublons", cost: "COÛT",
		total: "Total", tokens: "jetons",
		unpriced:   "événements sans prix connu ont été enregistrés à 0 $.",
		importWord: "Import", dryRun: "Simulation : rien n'a été écrit.",
	},
	"zh-CN": {
		status: "状态", files: "文件", missing: "缺失", ready: "就绪",
		scanned: "已扫描", new: "新增", dupes: "重复", cost: "费用",
		total: "合计", tokens: "令牌",
		unpriced:   "个事件没有已知价格，按 $0 记录。",
		importWord: "导入", dryRun: "试运行：未写入任何内容。",
	},
	"ja": {
		status: "状態", files: "ファイル", missing: "未検出", ready: "準備完了",
		scanned: "走査", new: "新規", dupes: "重複", cost: "費用",
		total: "合計", tokens: "トークン",
		unpriced:   "件のイベントは価格が不明なため $0 で記録されました。",
		importWord: "インポート", dryRun: "ドライラン: 何も書き込まれていません。",
	},
	"ar": {
		status: "الحالة", files: "الملفات", missing: "مفقود", ready: "جاهز",
		scanned: "مقروء", new: "جديد", dupes: "مكرر", cost: "التكلفة",
		total: "الإجمالي", tokens: "رموز",
		unpriced:   "أحداث بلا سعر معروف سُجلت بقيمة $0.",
		importWord: "استيراد", dryRun: "محاكاة: لم تُكتب أي بيانات.",
	},
	"it": {
		status: "STATO", files: "FILE", missing: "assente", ready: "pronto",
		scanned: "LETTI", new: "nuovi", dupes: "duplicati", cost: "COSTO",
		total: "Totale", tokens: "token",
		unpriced:   "eventi senza prezzo noto sono stati registrati a $0.",
		importWord: "Importazione", dryRun: "Simulazione: nulla è stato scritto.",
	},
	"ru": {
		status: "СТАТУС", files: "ФАЙЛЫ", missing: "нет", ready: "готов",
		scanned: "ПРОЧИТАНО", new: "новых", dupes: "дубликатов", cost: "СТОИМОСТЬ",
		total: "Итого", tokens: "токенов",
		unpriced:   "событий без известной цены записаны по $0.",
		importWord: "Импорт", dryRun: "Пробный запуск: ничего не записано.",
	},
	"uk": {
		status: "СТАН", files: "ФАЙЛИ", missing: "відсутній", ready: "готово",
		scanned: "ПРОЧИТАНО", new: "нових", dupes: "дублікатів", cost: "ВАРТІСТЬ",
		total: "Разом", tokens: "токенів",
		unpriced:   "подій без відомої ціни записано за $0.",
		importWord: "Імпорт", dryRun: "Пробний запуск: нічого не записано.",
	},
	"pl": {
		status: "STAN", files: "PLIKI", missing: "brak", ready: "gotowe",
		scanned: "ODCZYTANO", new: "nowych", dupes: "duplikatów", cost: "KOSZT",
		total: "Razem", tokens: "tokenów",
		unpriced:   "zdarzeń bez znanej ceny zapisano po $0.",
		importWord: "Import", dryRun: "Próbny przebieg: nic nie zapisano.",
	},
	"tr": {
		status: "DURUM", files: "DOSYA", missing: "yok", ready: "hazır",
		scanned: "TARANDI", new: "yeni", dupes: "yinelenen", cost: "MALİYET",
		total: "Toplam", tokens: "token",
		unpriced:   "olayın bilinen fiyatı yoktu ve $0 olarak kaydedildi.",
		importWord: "İçe aktarma", dryRun: "Deneme: hiçbir şey yazılmadı.",
	},
	"hi": {
		status: "स्थिति", files: "फ़ाइलें", missing: "अनुपलब्ध", ready: "तैयार",
		scanned: "स्कैन किए", new: "नए", dupes: "दोहरे", cost: "लागत",
		total: "कुल", tokens: "टोकन",
		unpriced:   "इवेंट का मूल्य अज्ञात था और $0 पर दर्ज किए गए।",
		importWord: "आयात", dryRun: "परीक्षण: कुछ भी नहीं लिखा गया।",
	},
	"id": {
		status: "STATUS", files: "BERKAS", missing: "tidak ada", ready: "siap",
		scanned: "DIPINDAI", new: "baru", dupes: "duplikat", cost: "BIAYA",
		total: "Total", tokens: "token",
		unpriced:   "peristiwa tanpa harga dikenal dicatat sebagai $0.",
		importWord: "Impor", dryRun: "Uji coba: tidak ada yang ditulis.",
	},
	"vi": {
		status: "TRẠNG THÁI", files: "TỆP", missing: "không có", ready: "sẵn sàng",
		scanned: "ĐÃ QUÉT", new: "mới", dupes: "trùng lặp", cost: "CHI PHÍ",
		total: "Tổng", tokens: "token",
		unpriced:   "sự kiện không có giá đã biết được ghi ở $0.",
		importWord: "Nhập", dryRun: "Chạy thử: không ghi gì.",
	},
	"th": {
		status: "สถานะ", files: "ไฟล์", missing: "ไม่พบ", ready: "พร้อม",
		scanned: "สแกนแล้ว", new: "ใหม่", dupes: "ซ้ำ", cost: "ค่าใช้จ่าย",
		total: "รวม", tokens: "โทเคน",
		unpriced:   "เหตุการณ์ที่ไม่ทราบราคาถูกบันทึกที่ $0",
		importWord: "นำเข้า", dryRun: "ทดลอง: ไม่ได้เขียนข้อมูลใด ๆ",
	},
	"zh-TW": {
		status: "狀態", files: "檔案", missing: "缺少", ready: "就緒",
		scanned: "已掃描", new: "新增", dupes: "重複", cost: "費用",
		total: "合計", tokens: "權杖",
		unpriced:   "筆事件沒有已知價格，按 $0 記錄。",
		importWord: "匯入", dryRun: "試執行：未寫入任何內容。",
	},
	"ko": {
		status: "상태", files: "파일", missing: "없음", ready: "준비됨",
		scanned: "스캔", new: "신규", dupes: "중복", cost: "비용",
		total: "합계", tokens: "토큰",
		unpriced:   "개의 이벤트는 알려진 가격이 없어 $0으로 기록되었습니다.",
		importWord: "가져오기", dryRun: "시험 실행: 아무것도 쓰지 않았습니다.",
	},
	"fa": {
		status: "وضعیت", files: "فایل‌ها", missing: "موجود نیست", ready: "آماده",
		scanned: "خوانده‌شده", new: "جدید", dupes: "تکراری", cost: "هزینه",
		total: "مجموع", tokens: "توکن",
		unpriced:   "رویداد بدون قیمت شناخته‌شده با $0 ثبت شد.",
		importWord: "درون‌ریزی", dryRun: "اجرای آزمایشی: چیزی نوشته نشد.",
	},
}

func cliLabelsFor(lang string) cliLabels {
	if labels, ok := cliLabelsByLocale[lang]; ok {
		return labels
	}
	if lang == "pt" {
		return cliLabelsByLocale["pt-BR"]
	}
	return cliLabelsEN
}

// cliLanguage resolves the language for human output: an explicit --lang wins,
// then the terminal locale, then English. Machine output never calls this.
func cliLanguage(explicit string) string {
	if explicit != "" {
		return explicit
	}
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		value := os.Getenv(name)
		if value == "" {
			continue
		}
		value = strings.SplitN(value, ".", 2)[0]
		value = strings.ReplaceAll(value, "_", "-")
		switch {
		case value == "C" || value == "POSIX":
			return "en"
		case value == "pt" || strings.HasPrefix(value, "pt-"):
			return "pt-BR"
		case strings.HasPrefix(value, "zh-"):
			// Chinese script matters: a Traditional tag must not collapse to
			// Simplified, which is what a bare `zh` would resolve to.
			if strings.Contains(strings.ToLower(value), "tw") ||
				strings.Contains(strings.ToLower(value), "hk") ||
				strings.Contains(strings.ToLower(value), "hant") {
				return "zh-TW"
			}
			return "zh-CN"
		}
		return strings.SplitN(value, "-", 2)[0]
	}
	return "en"
}
