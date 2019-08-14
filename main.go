package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/gojp/kana"
	"github.com/kennygrant/sanitize"
	"github.com/pkg/errors"
)

var chars = []rune(`▁▂▃▄▅▆▇█▏▎▍▌▋▊▉┼` +
	`┴┬┤├▔─│▕┌┐└┘╭╮╰╯` +
	` 。「」、・ヲァィゥェォャュョッ` +
	`ーアイウエオカキクケコサシスセソ` +
	`タチツテトナニヌネノハヒフヘホマ` +
	`ミムメモヤユヨラリルレロワン゛゜` +
	`═╞╪╡◢◣◥◤♠♥♦♣●○╱╲` +
	`╳円年月日時分秒        `)

var dakutens = []rune("かがきぎくぐけげこごさざしじすずせぜそぞただちぢつづてでとどはばひびふぶへべほぼうゔ" +
	"カガキギクグケゲコゴサザシジスズセゼソゾタダチヂツヅテデトドハバヒビフブヘベホボウヴ")

var handakutens = []rune("はぱひぴふぷへぺほぽ" +
	"ハパヒピフプヘペホポ")

func combineDakutens(s string) string {
	result := []rune{}
	for _, r := range []rune(s) {
		var table []rune
		if r == '゛' {
			table = dakutens
		}
		if r == '゜' {
			table = handakutens
		}
		n := len(result)
		if 0 < n && table != nil {
			for i := 0; i < len(table); i += 2 {
				if result[n-1] == table[i] {
					result[n-1] = table[i+1]
					r = 0
					break
				}
			}
		}
		if 0 < r {
			result = append(result, r)
		}
	}
	return string(result)
}

var categories = map[string]string{
	"Bass":          "bas チョッ",
	"Bells":         "bell",
	"Brass":         "brass brs horn sax",
	"Drums & Percs": "cowb perc drum dram drm timbal tom tam hihat hi-hat hi_hat kik kick タムタム dora ホ゛ンコ゛ ツツ゛ミ cow",
	"FX":            "laser train ufo car mushi sakebi efc tweet",
	"Keys":          "ep pian clav ap1",
	"Lead":          "main 7th",
	"Organ":         "orgn sin",
	"Pads":          "back down amb",
	"Plucked":       "guitar gtr koto harp banjo zitar",
	"Poly":          "",
	"Reed and Pipe": "flute oboe pic harm clari pipe",
	"Rhythmic":      "grock timp xylo vib",
	"Strings":       "str",
	"Synth":         "psg synt orc dgt",
	"Video Games":   "",
	"Woodwinds":     "kuchi fue",
}

var keywordToCategory = [][]string{}

func init() {
	for cate, s := range categories {
		if s == "" {
			continue
		}
		words := strings.Split(s, " ")
		for _, word := range words {
			keywordToCategory = append(keywordToCategory, []string{word, cate})
		}
	}
	sort.Slice(keywordToCategory, func(i int, j int) bool {
		return len(keywordToCategory[j][0]) < len(keywordToCategory[i][0])
	})
}

func guessCategory(name string) string {
	name = strings.ToLower(name)
	if name == "" {
		return "Video Games"
	}
	for _, k2c := range keywordToCategory {
		if strings.Contains(name, k2c[0]) {
			return k2c[1]
		}
	}
	return ""
}

var carriers = [][]bool{
	{false, false, false, true},
	{false, false, false, true},
	{false, false, false, true},
	{false, false, false, true},
	{false, true, false, true},
	{false, true, true, true},
	{false, true, true, true},
	{true, true, true, true},
}

type mucom88VoiceFormat [32]byte

func op2offset(op int) int {
	return (op&1)<<1 | (op&2)>>1
}

func (v mucom88VoiceFormat) name() string {
	var s = ""
	for i := 0; i < 6; i++ {
		b := v[26+i]
		if b == 0 {
			break
		}
		if 0x80 <= b {
			s += string(chars[b-0x80])
		} else {
			s += string(b)
		}
	}
	return strings.TrimSpace(s)
}

func (v mucom88VoiceFormat) sanitizedName() string {
	s := v.name()
	s = combineDakutens(s)
	s = kana.KanaToRomaji(s)
	return sanitize.BaseName(s)
}

func (v mucom88VoiceFormat) patchName(pc int) string {
	name := v.sanitizedName()
	if name != "" {
		name = "-" + name
	}
	return fmt.Sprintf("MUCOM88-%03d%s", pc, name)
}

func (v mucom88VoiceFormat) category() string {
	cate := guessCategory(v.name())
	if cate != "" {
		return cate
	}
	log.Printf("could not guess category: %s", v.name())
	return "Lead"
}

func (v mucom88VoiceFormat) ml(op int) int {
	return int(v[1+op2offset(op)] & 15)
}

func (v mucom88VoiceFormat) dt(op int) int {
	return int(v[1+op2offset(op)] >> 4 & 7)
}

func (v mucom88VoiceFormat) tl(op int) int {
	return int(v[5+op2offset(op)] & 127)
}

func (v mucom88VoiceFormat) ar(op int) int {
	return int(v[9+op2offset(op)] & 31)
}

func (v mucom88VoiceFormat) ks(op int) int {
	return int(v[9+op2offset(op)] >> 6)
}

func (v mucom88VoiceFormat) dr(op int) int {
	return int(v[13+op2offset(op)] & 31)
}

func (v mucom88VoiceFormat) am(op int) int {
	return int(v[13+op2offset(op)] >> 7)
}

func (v mucom88VoiceFormat) sr(op int) int {
	return int(v[17+op2offset(op)] & 31)
}

func (v mucom88VoiceFormat) rr(op int) int {
	return int(v[21+op2offset(op)] & 15)
}

func (v mucom88VoiceFormat) sl(op int) int {
	return int(v[21+op2offset(op)] >> 4)
}

func (v mucom88VoiceFormat) al() int {
	return int(v[25] & 7)
}

func (v mucom88VoiceFormat) fb() int {
	return int(v[25] >> 3 & 7)
}

func (v mucom88VoiceFormat) print() {
	fmt.Printf("Name: %q\n", v.name())
	fmt.Printf("  Algorithm %d\n", v.al()) // 0..7
	fmt.Printf("  Feedback  %d\n", v.fb()) // 0..7
	for op := 0; op < 4; op++ {
		fmt.Printf("  Op.%d\n", op+1)
		fmt.Printf("    Attack R.   %2d\n", v.ar(op)) // 0..31
		fmt.Printf("    Decay R.    %2d\n", v.dr(op)) // 0..31
		fmt.Printf("    Sustain R.  %2d\n", v.sr(op)) // 0..31
		fmt.Printf("    Release R.  %2d\n", v.rr(op)) // 0..15
		fmt.Printf("    Sus.Level   %2d\n", v.sl(op)) // 0..15
		fmt.Printf("    Total Level %2d\n", v.tl(op)) // 0..127
		fmt.Printf("    KeyScale R. %2d\n", v.ks(op)) // 0..3
		fmt.Printf("    Multiple    %2d\n", v.ml(op)) // 0..15
		fmt.Printf("    Detune      %2d\n", v.dt(op)) // 0..7
		fmt.Printf("    AM          %2d\n", v.am(op)) // 0..1
	}
}

func (v mucom88VoiceFormat) mucomText(pc int) string {
	b := new(strings.Builder)
	fmt.Fprintf(b, "  @%d:{\n", pc)
	fmt.Fprintf(b, "%4d,%3d", v.fb(), v.al())
	for op := 0; op < 4; op++ {
		fmt.Fprintln(b, "")
		fmt.Fprintf(b, " %3d", v.ar(op))
		fmt.Fprintf(b, ",%3d", v.dr(op))
		fmt.Fprintf(b, ",%3d", v.sr(op))
		fmt.Fprintf(b, ",%3d", v.rr(op))
		fmt.Fprintf(b, ",%3d", v.sl(op))
		fmt.Fprintf(b, ",%3d", v.tl(op))
		fmt.Fprintf(b, ",%3d", v.ks(op))
		fmt.Fprintf(b, ",%3d", v.ml(op))
		fmt.Fprintf(b, ",%3d", v.dt(op))
	}
	fmt.Fprintf(b, ",%q}\n\n", v.name())
	return b.String()
}

var muls = []int{0, 1054, 1581, 2635, 3689, 4743, 5797, 6851, 7905, 8959, 10013, 10540, 11594, 12648, 14229, 15000}

func (v mucom88VoiceFormat) toRYM2612(pc int) string {
	buf := make([][]string, 4)
	for op := 0; op < 4; op++ {
		i := op + 1
		dt := v.dt(op)
		if 4 <= dt {
			dt = 4 - dt
		}
		tl := 127 - v.tl(op)
		vel := 0
		if carriers[v.al()][op] {
			vel = tl / 2
			tl -= vel
		}
		buf[op] = []string{}
		buf[op] = append(buf[op], fmt.Sprintf(`  <PARAM id="OP%dVel" value="%.1f"/>`, i, float64(vel)))
		buf[op] = append(buf[op], fmt.Sprintf(`  <PARAM id="OP%dTL" value="%.1f"/>`, i, float64(tl))) // 0..127 -> 127..0
		buf[op] = append(buf[op], fmt.Sprintf(`  <PARAM id="OP%dSSGEG" value="0.0"/>`, i))
		buf[op] = append(buf[op], fmt.Sprintf(`  <PARAM id="OP%dRS" value="%.1f"/>`, i, float64(v.ks(op)))) // 0..3
		buf[op] = append(buf[op], fmt.Sprintf(`  <PARAM id="OP%dRR" value="%.1f"/>`, i, float64(v.rr(op)))) // 0..15
		buf[op] = append(buf[op], fmt.Sprintf(`  <PARAM id="OP%dMW" value="0.0"/>`, i))
		buf[op] = append(buf[op], fmt.Sprintf(`  <PARAM id="OP%dMUL" value="%.1f"/>`, i, float64(muls[v.ml(op)]))) // 0..15 -> 0..15000
		buf[op] = append(buf[op], fmt.Sprintf(`  <PARAM id="OP%dFixed" value="0.0"/>`, i))
		buf[op] = append(buf[op], fmt.Sprintf(`  <PARAM id="OP%dDT" value="%.1f"/>`, i, float64(dt)))           // 0..7 -> -3..3
		buf[op] = append(buf[op], fmt.Sprintf(`  <PARAM id="OP%dD2R" value="%.1f"/>`, i, float64(v.sr(op))))    // 0..31
		buf[op] = append(buf[op], fmt.Sprintf(`  <PARAM id="OP%dD2L" value="%.1f"/>`, i, float64(15-v.sl(op)))) // 0..15 -> 15..0
		buf[op] = append(buf[op], fmt.Sprintf(`  <PARAM id="OP%dD1R" value="%.1f"/>`, i, float64(v.dr(op))))    // 0..31
		buf[op] = append(buf[op], fmt.Sprintf(`  <PARAM id="OP%dAR" value="%.1f"/>`, i, float64(v.ar(op))))     // 0..31
		buf[op] = append(buf[op], fmt.Sprintf(`  <PARAM id="OP%dAM" value="%.1f"/>`, i, float64(v.am(op))))     // 0..1
	}

	b := new(strings.Builder)
	fmt.Fprintln(b, `<?xml version="1.0" encoding="UTF-8"?>`)
	fmt.Fprintln(b, ``)
	fmt.Fprintf(b, `<RYM2612Params patchName="%s" category="%s" rating="3" type="User">%s`, v.patchName(pc), v.category(), "\n")
	for i := 0; i < len(buf[0]); i++ {
		for op := 3; 0 <= op; op-- {
			fmt.Fprintln(b, buf[op][i])
		}
	}
	fmt.Fprintln(b, `  <PARAM id="volume" value="0.4483062326908112"/>`)
	fmt.Fprintln(b, `  <PARAM id="Ladder_Effect" value="0.0"/>`)
	fmt.Fprintln(b, `  <PARAM id="Output_Filtering" value="0.0"/>`)
	fmt.Fprintln(b, `  <PARAM id="Polyphony" value="8.0"/>`)
	fmt.Fprintln(b, `  <PARAM id="TimerA" value="0.2000000029802322"/>`)
	fmt.Fprintln(b, `  <PARAM id="Spec_Mode" value="2.0"/>`)
	fmt.Fprintln(b, `  <PARAM id="Pitchbend_Range" value="2.0"/>`)
	fmt.Fprintln(b, `  <PARAM id="Legato_Retrig" value="0.0"/>`)
	fmt.Fprintln(b, `  <PARAM id="LFO_Speed" value="2.0"/>`)
	fmt.Fprintln(b, `  <PARAM id="LFO_Enable" value="1.0"/>`)
	fmt.Fprintf(b, `  <PARAM id="Feedback" value="%.1f"/>%s`, float64(v.fb()), "\n")
	fmt.Fprintln(b, `  <PARAM id="FMSMW" value="100.0"/>`)
	fmt.Fprintln(b, `  <PARAM id="FMS" value="0.0"/>`)
	fmt.Fprintln(b, `  <PARAM id="DAC_Prescaler" value="1.0"/>`)
	fmt.Fprintf(b, `  <PARAM id="Algorithm" value="%.1f"/>%s`, float64(1+v.al()), "\n")
	fmt.Fprintln(b, `  <PARAM id="AMS" value="0.0"/>`)
	fmt.Fprintln(b, `  <PARAM id="masterTune"/>`)
	fmt.Fprintln(b, `</RYM2612Params>`)
	return b.String()
}

func convert(src, dst string) error {
	b, err := ioutil.ReadFile(src)
	if err != nil {
		return errors.WithStack(err)
	}

	if err := os.MkdirAll(dst, 0755); err != nil {
		return errors.WithStack(err)
	}

	pc := -1
	var empty []string
	for i := 0; i+32 <= len(b); i += 32 {
		pc++
		var v mucom88VoiceFormat
		copy(v[:], b[i:i+32])
		name := v.patchName(pc)
		filename := filepath.Join(dst, name+".rym2612")
		xml := v.toRYM2612(pc)
		err := ioutil.WriteFile(filename, []byte(xml), 0644)
		if err != nil {
			return errors.WithStack(err)
		}
		if v.name() == "" {
			empty = append(empty, filename)
		} else {
			empty = nil
		}
	}
	for _, filename := range empty {
		_ = os.Remove(filename)
	}
	return nil
}

func main() {
	err := convert("voice.dat", "output")
	if err != nil {
		panic(err)
	}
}
