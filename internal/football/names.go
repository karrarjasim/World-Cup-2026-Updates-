package football

import "strings"

var arNames = map[string]string{
	"qatar": "قطر", "ecuador": "الإكوادور", "senegal": "السنغال", "netherlands": "هولندا",
	"england": "إنجلترا", "iran": "إيران", "usa": "الولايات المتحدة", "united states": "الولايات المتحدة",
	"wales": "ويلز", "argentina": "الأرجنتين", "saudi arabia": "السعودية", "mexico": "المكسيك",
	"poland": "بولندا", "france": "فرنسا", "australia": "أستراليا", "denmark": "الدنمارك",
	"tunisia": "تونس", "spain": "إسبانيا", "costa rica": "كوستاريكا", "germany": "ألمانيا",
	"japan": "اليابان", "belgium": "بلجيكا", "canada": "كندا", "morocco": "المغرب",
	"croatia": "كرواتيا", "brazil": "البرازيل", "serbia": "صربيا", "switzerland": "سويسرا",
	"cameroon": "الكاميرون", "portugal": "البرتغال", "ghana": "غانا", "uruguay": "الأوروغواي",
	"south korea": "كوريا الجنوبية", "korea republic": "كوريا الجنوبية", "north korea": "كوريا الشمالية",
	"algeria": "الجزائر", "egypt": "مصر", "iraq": "العراق", "jordan": "الأردن",
	"united arab emirates": "الإمارات", "uae": "الإمارات", "oman": "عُمان", "bahrain": "البحرين",
	"kuwait": "الكويت", "palestine": "فلسطين", "lebanon": "لبنان", "syria": "سوريا",
	"italy": "إيطاليا", "colombia": "كولومبيا", "chile": "تشيلي", "peru": "بيرو",
	"nigeria": "نيجيريا", "ivory coast": "ساحل العاج", "cote d'ivoire": "ساحل العاج",
	"south africa": "جنوب أفريقيا", "scotland": "اسكتلندا", "norway": "النرويج", "sweden": "السويد",
	"austria": "النمسا", "turkey": "تركيا", "türkiye": "تركيا", "greece": "اليونان",
	"new zealand": "نيوزيلندا", "paraguay": "باراغواي", "venezuela": "فنزويلا", "bolivia": "بوليفيا",
	"panama": "بنما", "jamaica": "جامايكا", "honduras": "هندوراس", "uzbekistan": "أوزبكستان",
	"china": "الصين", "india": "الهند", "indonesia": "إندونيسيا", "mali": "مالي",
	"cape verde": "الرأس الأخضر", "ukraine": "أوكرانيا", "russia": "روسيا", "czech republic": "التشيك",
	"romania": "رومانيا", "hungary": "المجر", "slovakia": "سلوفاكيا", "slovenia": "سلوفينيا",
	"czechia": "التشيك", "congo dr": "الكونغو الديمقراطية", "dr congo": "الكونغو الديمقراطية",
	"bosnia-herzegovina": "البوسنة والهرسك", "bosnia and herzegovina": "البوسنة والهرسك",
	"curaçao": "كوراساو", "curacao": "كوراساو",
}

func ArName(en string) string {
	if v, ok := arNames[strings.ToLower(strings.TrimSpace(en))]; ok {
		return v
	}
	return en
}
