# راهنمای کامپایل و مستندات فنی

> [English version](BUILD.md)

این سند برای **توسعه‌دهندگانی** است که می‌خواهند از سورس کامپایل کنند، معماری را
بفهمند، یا برنامه را گسترش دهند. برای راهنمای نصب کاربران [README.fa.md](README.fa.md) را ببینید.

---

## فهرست

- [کامپایل سریع](#کامپایل-سریع)
- [اسکریپت‌های ساخت (همه پلتفرم‌ها)](#اسکریپت‌های-ساخت-همه-پلتفرم‌ها)
- [کامپایل کراس‌پلتفرم](#کامپایل-کراس‌پلتفرم)
- [پیش‌نیازها](#پیش‌نیازها)
- [ساختار پروژه](#ساختار-پروژه)
- [معماری](#معماری)
- [مرجع API](#مرجع-api)
- [پارس کانفیگ](#پارس-کانفیگ)
- [نویز UDP](#نویز-udp)
- [فرانت‌اند (Svelte UI)](#فرانت‌اند-svelte-ui)
- [تست](#تست)
- [CI/CD](#cicd)
- [متغیرهای محیطی](#متغیرهای-محیطی)
- [نکات پلتفرم](#نکات-پلتفرم)
- [مشارکت](#مشارکت)

---

## کامپایل سریع

```bash
git clone https://github.com/QMahyar/Cloudflare-Scanner.git
cd Cloudflare-Scanner
go build -ldflags="-s -w -X 'main.Version=dev'" -o Cloudflare-Scanner .
```

فایل باینری فرانت‌اند کامپایل‌شده (`ui/dist/`، یک SPA با Vite + Svelte 5 — به
[فرانت‌اند (Svelte UI)](#فرانت‌اند-svelte-ui) مراجعه کنید) را از طریق دستور
`//go:embed all:ui/dist` در گو توکار می‌کند. `ui/dist/` در ریپو commit شده،
پس `go build` ساده نیازی به Node ندارد — فقط هنگام تغییر `frontend/src/` آن را
با `npm run build` بازسازی کنید.

## اسکریپت‌های ساخت (همه پلتفرم‌ها)

پوشه `scripts/` شامل دو اسکریپت کامل است که دقیقاً همان کاری را می‌کنند که CI انجام می‌دهد:
Go را در صورت نبود نصب می‌کند، باینری را کامپایل می‌کند، xray-core مربوطه را دانلود می‌کند
و آرشیو نهایی تولید می‌کند.

### لینوکس / مک / Termux — `scripts/build.sh`

```bash
# ساخت برای پلتفرم میزبان (تشخیص خودکار)
./scripts/build.sh

# ساخت همه پلتفرم‌های پشتیبانی‌شده
./scripts/build.sh all

# ساخت یک یا چند پلتفرم مشخص
./scripts/build.sh linux-amd64
./scripts/build.sh linux-amd64 darwin-arm64
```

### ویندوز — `scripts/build.ps1`

```powershell
# ساخت برای پلتفرم میزبان (تشخیص خودکار)
.\scripts\build.ps1

# ساخت همه پلتفرم‌های پشتیبانی‌شده
.\scripts\build.ps1 all

# ساخت یک یا چند پلتفرم مشخص
.\scripts\build.ps1 windows-amd64
.\scripts\build.ps1 windows-amd64 linux-amd64
```

### کلیدهای پلتفرم پشتیبانی‌شده

| کلید | سیستم‌عامل | معماری |
|------|-----------|-------|
| `windows-amd64` | ویندوز | x86-64 |
| `windows-arm64` | ویندوز | ARM64 |
| `linux-amd64` | لینوکس | x86-64 |
| `linux-arm64` | لینوکس / Raspberry Pi | ARM64 |
| `termux-arm64` | اندروید (Termux) | ARM64 |
| `darwin-amd64` | مک | Intel |
| `darwin-arm64` | مک | Apple Silicon |

### متغیرهای محیطی

| متغیر | پیش‌فرض | توضیح |
|-------|---------|-------|
| `VERSION` | `git describe --tags` | نسخه‌ای که در باینری ذخیره می‌شود |
| `XRAY_VERSION` | `v1.8.24` | نسخه xray-core برای بسته‌بندی |
| `NO_XRAY=1` | — | بدون دانلود xray (فقط باینری) |
| `NO_ARCHIVE=1` | — | فایل‌های loose در `dist/<platform>/` بدون آرشیو |
| `GO_VERSION` | `1.26.2` | نسخه Go برای نصب خودکار در صورت نبود |

### خروجی اسکریپت‌ها

```
dist/
├── windows-amd64/
│   ├── Cloudflare-Scanner.exe
│   └── xray.exe
├── linux-amd64/
│   ├── Cloudflare-Scanner
│   └── xray
├── Cloudflare-Scanner-v3.0.1-windows-amd64.zip
├── Cloudflare-Scanner-v3.0.1-linux-amd64.tar.gz
└── ...
```

خروجی‌ها از نظر ساختاری با دانلودهای GitHub Release یکسان هستند.

## کامپایل کراس‌پلتفرم

```bash
# ویندوز (amd64)
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o Cloudflare-Scanner.exe .

# لینوکس (amd64)
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o Cloudflare-Scanner .

# لینوکس (arm64) — Raspberry Pi و غیره
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o Cloudflare-Scanner .

# مک (Intel)
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o Cloudflare-Scanner .

# مک (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o Cloudflare-Scanner .
```

در PowerShell: `$env:GOOS="windows"; $env:GOARCH="amd64"; go build ...`

## پیش‌نیازها

- **Go 1.26+** ([go.dev](https://go.dev/dl/))
- **بدون کامپایلر C** — همه وابستگی‌ها pure Go
- کراس‌پلتفرم: ویندوز، لینوکس، مک، Termux/اندروید

---

## ساختار پروژه

```
Cloudflare-Scanner/
├── main.go           # نقطه ورودی — بررسی xray، شروع سرور، باز کردن مرورگر
├── server.go         # هندلرهای HTTP، jobهای اسکن، API، توکار کردن UI (//go:embed all:ui/dist)
├── about.go          # /api/version و /api/update-check (انتشارهای گیت‌هاب)
├── config.go         # پارسر کانفیگ WireGuard (استاندارد + Hogwarts)
├── endpoint.go       # تولیدکننده اندپوینت تصادفی WARP
├── scanner.go        # اسکنر اندپوینت — هندشیک WireGuard بومی + fallback نویز xray
├── warp_probe.go     # پروب هندشیک WireGuard بومی (Noise_IKpsk2)
├── xray.go           # سازنده کانفیگ xray-core (WireGuard outbound, SOCKS5 inbound)
├── cleanip.go        # اسکنر آی‌پی تمیز — تولید CIDR، پروب TCP، اعتبارسنجی xray، اسکن نزدیک
├── iprange.go        # پارس/تولید بازه‌های IP سفارشی برای اسکنر آی‌پی
├── replacer.go       # دریافت اشتراک، یکتاسازی کانفیگ، جایگزینی دسته‌جمعی IP
├── proxy.go          # پارسر URL VLESS/Trojan/VMess، سازنده کانفیگ xray، سازنده share URL
├── metrics.go        # توابع کیفیت اسکن (میانه، بهترین، جیتر، مرتب‌سازی)
├── noise.go          # کانفیگ نویز UDP و اعتبارسنجی
├── parsers_test.go   # تست‌های واحد برای پارسینگ، جایگزین، مسیر traversل
├── cleanip_test.go   # تست‌های تولید آی‌پی تمیز
├── iprange_test.go   # تست‌های پارس بازه‌های IP سفارشی
├── replacer_name_test.go # تست‌های قالب نام‌گذاری کانفیگ
├── warp_probe_test.go    # تست‌های پروب هندشیک WireGuard بومی
├── frontend/         # سورس SPA با Vite + Svelte 5 (به فرانت‌اند (Svelte UI) مراجعه کنید)
│   └── src/
│       ├── components/  # یک فایل *.svelte به ازای هر تب
│       ├── lib/          # توابع مشترک (api, sse, stores, i18n, ...)
│       └── locales/      # en.json / fa.json (svelte-i18n)
├── ui/
│   └── dist/         # باندل ساخته‌شده فرانت‌اند، commit شده و توکار در گو
├── scripts/          # اسکریپت‌های ساخت + نصب یک‌خطی هر پلتفرم
├── docs/             # راهنماهای کاربری (انگلیسی + فارسی)
├── README.md         # مستندات انگلیسی کاربری
├── README.fa.md      # مستندات فارسی کاربری
├── BUILD.md          # راهنمای توسعه انگلیسی
├── BUILD.fa.md       # این فایل
├── .github/workflows/ # CI (go + frontend) و Release
├── AGENTS.md         # دستورالعمل‌های Agent/Copilot
├── CHANGELOG.md      # تاریخچه انتشار
├── LICENSE           # MIT
└── sample.conf       # نمونه کانفیگ WireGuard
```

---

## معماری

### جریان اسکن اندپوینت

```
آپلود .conf توسط کاربر ──>  ParseWarpConfig()
                                  │
                    IP:port تصادفی │ (پرافیکس‌ها و پورت‌ها از endpoint.go)
                                  ▼
                           runScan() → Scanner(testEndpointAttempts)
                                           │
                             ┌──────────────┼──────────────┐
                             ▼              ▼              ▼
                        Worker 1        Worker 2 ...  Worker N
                             │              │              │
                        testEndpointAttempts (۲ تلاش)
                        └─ بدون نویز: WarpHandshakeProbe() هندشیک UDP بومی
                        └─ با نویز: xray WG outbound → SOCKS5 → HTTP GET
                        └─ میانه تأخیر در تلاش‌ها
                             │              │              │
                             └──────────────┴──────────────┘
                                             │
                                       sortScanResults()
                                       (اول موفق، سپس تأخیر)
                                             │
                                       بازگشت نتایج
```

### جریان اسکن آی‌پی تمیز

```
ارائه VLESS URL توسط کاربر        CleanIPGenerator
       (اختیاری)                      └─ GenerateIPs()
                                         └─ ۲۵ CIDR IPv4 + ۹۱ CIDR IPv6
                                         └─ توزیع وزنی تصادفی
                                              │
                                       فاز ۱: پروب TCP
                                         └─ ۵۰۰ کارگر همزمان
                                         └─ net.DialTimeout(ip:port, ۳s)
                                         └─ probeCloudflareTrace() برای colo/loc
                                              │
                                 ┌───────────┴──────────┐
                                 ▼                      ▼
                           SkipPhase2=true        SkipPhase2=false
                                 │                      │
                            تمام                    فاز ۲: اعتبارسنجی xray
                                                      └─ sortCleanIPResults()
                                                      └─ گرفتن N نامزد برتر
                                                      └─ validateWithXrayAttempts (۲ تلاش)
                                                      └─ هرکدام: کانفیگ xray → SOCKS5 → HTTP GET
                                                      └─ میانه تأخیر، بهترین، جیتر
                                                           │
                                                      GenerateExport()
                                                      └─ URLهای VLESS یا لیست IP:port
```

### جریان جایگزین آی‌پی

```
URL اشتراک ──>  FetchSubscription()
                 └─ اعتبارسنجی scheme → HTTP GET → محدودیت ۱۰ مگابایت
                 └─ base64 decode → ParseSubscription()
                      │
                 DeduplicateConfigs()
                 └─ اثرانگشت: protocol, UUID, encryption, security, ...
                 └─ بازگشت قالب‌های کانفیگ یکتا
                      │
                 کاربر کانفیگ‌ها را انتخاب + paste اندپوینت‌ها
                      │
                 GenerateReplacedConfigs(configs, endpoints)
                 └─ ضرب دکارتی: هر کانفیگ × هر اندپوینت
                 └─ حذف تکراری (همان اثرانگشت + همان اندپوینت)
                 └─ اضافه کردن " @ endpoint" به remark
                 └─ بازگشت URLهای VLESS
```

---

## مرجع API

همه اندپوینت‌ها از `127.0.0.1:{port_random}` سرو می‌شوند. API برای UI توکار
طراحی شده — بدون احراز هویت.

### اسکنر اندپوینت

| اندپوینت | متد | توضیح |
|----------|--------|-------------|
| `/` | GET | سرو SPA توکار Svelte (`ui/dist/index.html` + assets) |
| `/api/scan` | POST | شروع اسکن اندپوینت WARP |
| `/api/status/{id}` | GET | `{status, progress, total}` |
| `/api/results/{id}` | GET | `{entries[], raw[], failures[], status}` |
| `/api/stop/{id}` | POST | لغو اسکن در حال اجرا |
| `/api/apply-endpoint` | POST | اعمال اندپوینت به فایل‌های .conf آپلودی |

### اسکنر آی‌پی تمیز

| اندپوینت | متد | توضیح |
|----------|--------|-------------|
| `/api/clean-scan` | POST | شروع اسکن آی‌پی تمیز |
| `/api/clean-status/{id}` | GET | وضعیت فاز ۱ و ۲ |
| `/api/clean-results/{id}` | GET | نتایج فاز ۱ و ۲ |
| `/api/clean-stop/{id}` | POST | توقف اسکن |
| `/api/clean-export` | POST | تولید URL از VLESS URL + اندپوینت‌ها |

### جایگزین آی‌پی

| اندپوینت | متد | توضیح |
|----------|--------|-------------|
| `/api/replacer/fetch` | POST | دریافت + یکتاسازی از URL اشتراک |
| `/api/replacer/parse` | POST | پارس متن خام به کانفیگ |
| `/api/replacer/apply` | POST | تولید کانفیگ‌های جایگزین شده |

---

## پارس کانفیگ

### WireGuard (.conf)

پشتیبانی از دو فرمت استاندارد و Hogwarts.

**استاندارد:**

```ini
[Interface]
PrivateKey = ...
Address = 2606:4700:110:.../128
Reserved = 0,0,0
MTU = 1280

[Peer]
PublicKey = bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=
Endpoint = 162.159.192.1:2408
```

**Hogwarts:**

```ini
[Interface]
PrivateKey = ...
Address = ...
S1 = 0
S2 = 0
S3 = 0
Jc = ...   ; نادیده گرفته می‌شود
Jmin = ... ; نادیده گرفته می‌شود

[Peer]
PublicKey = ...
Endpoint = ...
```

پارسر کلیدها را به lowercase تبدیل می‌کند. S1/S2/S3 Reserved را پر می‌کنند
وقتی فیلد `Reserved` وجود نداشته باشد.

### URL اشتراک VLESS/Trojan

با `url.Parse` پارس می‌شود. پشتیبانی از IPv4، IPv6 (باکرت)، تمام پارامترهای
استاندارد: `security`, `sni`, `fp`, `type`, `host`, `path`, `flow`,
`pbk`, `sid`, `spx`, `allowInsecure`, `alpn`, `headerType`, `mode`,
`serviceName`, `packetEncoding`.

---

## نویز UDP

وقتی فعال باشد، xray-core بسته‌های تصادفی UDP قبل از دست دادن WireGuard می‌فرستد
با تأخیر تصادفی. این کار بلاک DPI ترافیک استاندارد WARP را دور می‌زند.

| پارامتر | پیش‌فرض | محدوده | توضیح |
|---------|---------|--------|-------|
| Type | `rand` | rand / base64 / hex / str | نوع محتوای بسته نویز |
| Packet | `50-100` | ۱-۱۵۰۰ بایت | اندازه یا محدوده مقدار بسته |
| Delay | `1-5` | ۱-۱۰۰۰ ms | تأخیر بین بسته‌های نویز |
| Count | ۵ | ۱-۵۰ | تعداد بسته‌های نویز در هر دست دادن |

---

## فرانت‌اند (Svelte UI)

رابط کاربری یک SPA با Vite + Svelte 5 در `frontend/` است. باندل commit‌شده
`ui/dist/` همان چیزی است که `go build` توکار می‌کند، پس **تغییر UI یک فرایند
دو مرحله‌ای است**: ابتدا `ui/dist/` را بازسازی کنید، سپس باینری گو را.

```bash
cd frontend
npm install        # فقط بار اول
npm run dev        # سرور توسعه با hot-reload
npm run build      # بازسازی ../ui/dist — این را همراه تغییرتان commit کنید
cd ..
go build -ldflags="-s -w -X 'main.Version=dev'" -o Cloudflare-Scanner .
```

ساختار:

- `src/components/` — یک فایل `*.svelte` به ازای هر تب (`EndpointScanner`,
  `IpScanner`, `Replacer`, `About`) به‌علاوه ویجت‌های مشترک (`VirtualTable`,
  `SplitCopyButton`, `QrModal`, ...).
- `src/lib/` — توابع مشترک: `api.js` (wrapper برای fetch)، `sse.js` (استریم
  وضعیت)، `stores.js` (تنظیمات/نتایج پایدار)، `handoff.js` (انتقال بین تب‌ها)،
  `sort.js`, `copymode.js` و غیره.
- `src/locales/en.json` و `fa.json` — رشته‌های UI از طریق `svelte-i18n`، با
  کلیدهای یکسان. برای هر تغییر متن کاربرپسند، **هر دو** فایل را به‌روز کنید.

برای `npm run dev`، سرور گو را هم اجرا کنید (`go run .`) و پورت تصادفی چاپ‌شده
را یادداشت کنید، سپس پراکسی توسعه Vite را به آن آدرس بدهید تا فراخوانی‌های
`/api/*` پاسخ بگیرند:

```bash
VITE_API_TARGET=http://127.0.0.1:<port> npm run dev   # به frontend/vite.config.js مراجعه کنید
```

## تست

```bash
go vet ./...
go test ./...       # تست‌های پارسینگ، امنیت، تولید
go test -race ./... # اختیاری — کندتر، race condition را می‌یابد
```

تست‌ها پوشش می‌دهند:

- پارس کانفیگ WARP
- پارس URL VLESS/Trojan
- یکتاسازی و تولید ضرب دکارتی
- منطق جلوگیری از path traversal
- تولید CIDR آی‌پی تمیز

---

## CI/CD

### CI — Go (`.github/workflows/ci.yml`)

اجرا در هر push/PR به `master`. ماتریس: ۶ ترکیب پلتفرم/معماری.

مراحل: `go vet` ← `go test` ← `go build`

### CI — فرانت‌اند (`.github/workflows/frontend.yml`)

فقط وقتی `frontend/**` یا `ui/dist/**` تغییر کند اجرا می‌شود (paths-filtered،
پس push-های فقط-Go آن را اجرا نمی‌کنند). باندل Svelte را دوباره می‌سازد و
تازگی `ui/dist/` را بررسی می‌کند.

### Release (`.github/workflows/release.yml`)

با `git tag v*` یا دستی از GitHub UI فعال می‌شود. ۷ هدف می‌سازد، xray-core
متناظر را دانلود می‌کند، آرشیو `.zip`/`.tar.gz` می‌سازد، checksum تولید می‌کند
و در GitHub Releases منتشر می‌کند.

---

## متغیرهای محیطی

هیچ کدام لازم نیست. همه چیز از طریق UI وب در زمان اجرا تنظیم می‌شود.

---

## نکات پلتفرم

### لینوکس

- مرورگر از طریق `xdg-open` باز می‌شود — در صورت نیاز `xdg-utils` را نصب کنید.
- `chmod +x xray` بعد از استخراج الزامی است.

### Termux / اندروید

- نسخه release شامل xray-core مخصوص اندروید است.
- مرورگر از طریق `termux-open-url` باز می‌شود — `pkg install termux-open-url`.
- اسکریپت: `scripts/termux-setup.sh` برای نصب یک‌خطی.

### مک

- Gatekeeper ممکن است `xray` را بلاک کند — `xattr -dr com.apple.quarantine xray`.
- اگر خود برنامه بلاک شد: System Settings → Privacy & Security → Open Anyway.

### ویندوز

- آنتی‌ویروس ممکن است `xray.exe` را بلاک کند — برای پوشه استثنا تعریف کنید.
- در سیستم‌های محدود شده، برنامه را به عنوان Administrator اجرا کنید.

---

## مشارکت

۱. Fork کنید
۲. branch ایجاد کنید (`git checkout -b my-feature`)
۳. تغییرات را اعمال کنید، `go vet ./...` و `go test ./...` اجرا کنید
۴. کامپایل و تست محلی: `go build -o Cloudflare-Scanner.exe .`
۵. commit با پیام واضح
۶. push و PR باز کنید

---

## لایسنس

MIT — استفاده، تغییر و توزیع آزادانه.
