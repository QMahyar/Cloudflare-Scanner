# راهنمای کامپایل و مستندات فنی

> [English version](BUILD.md)

این سند شامل ساختار کامل پروژه، معماری، دستورات کامپایل و مرجع API برای هر سه ابزار است: **اسکنر اندپوینت** (واپ)، **اسکنر آی‌پی تمیز** (Clean IP) و **جایگزین آی‌پی** (IP Replacer). برای توسعه‌دهندگانی که از سورس کامپایل می‌کنند، برنامه را گسترش می‌دهند یا می‌خواهند اجزای پروژه را درک کنند مفید است.

## پیش‌نیازها

- **Go 1.26+** (دانلود از [go.dev](https://go.dev/dl/))
- **کامپایلر C لازم نیست** — تمام وابستگی‌ها خالص Go هستند
- کراس پلتفرم: ویندوز، لینوکس، مک (تمامی معماری‌ها)

## دستورات کامپایل

```bash
# ویندوز (amd64)
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o Cloudflare-Scanner.exe .

# لینوکس (amd64)
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o Cloudflare-Scanner .

# لینوکس (arm64) — مثلاً Raspberry Pi
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o Cloudflare-Scanner .

# مک (Intel)
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o Cloudflare-Scanner .

# مک (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o Cloudflare-Scanner .
```

در **ویندوز (PowerShell)** دستورات مشابه با `$env:GOOS="windows"; $env:GOARCH="amd64"; go build ...` کار می‌کنند.

فایل باینری `ui/index.html` را از طریق دستور `//go:embed` گو در خود جاسازی می‌کند — نیازی به فایل‌های خارجی در زمان اجرا نیست.

## ساختار پروژه

```
Cloudflare-Scanner/
├── main.go          # نقطه ورود، بررسی وجود xray.exe، راه‌اندازی سرور HTTP
├── server.go        # هندلرهای HTTP (اسکن، وضعیت، نتایج، توقف، اعمال اندپوینت، اسکن تمیز، جایگزین)
├── config.go        # پارسر کانفیگ WireGuard (case-insensitive، پشتیبانی از سبک هاگوارتز)
├── endpoint.go      # تولیدکننده تصادفی اندپوینت‌های IPv4/IPv6 کلاودفلر
├── scanner.go       # تست‌کننده موازی اندپوینت (۱۲ کارگر، دست دادن SOCKS5)
├── xray.go          # تولیدکننده کانفیگ JSON ایکس‌ری و مدیریت فرآیند
├── noise.go         # کانفیگ نویز UDP و اعتبارسنجی
├── proxy.go         # پارسر لینک اشتراک VLESS/Trojan و سازنده کانفیگ JSON ایکس‌ری
├── cleanip.go       # اسکنر آی‌پی تمیز: تولید آی‌پی بر اساس CIDR، پروب TCP، اعتبارسنجی xray، خروجی
├── replacer.go      # دریافت کننده اشتراک، یکتا کننده، جایگزین IP به صورت ضرب دکارتی
├── cleanip_test.go  # تست‌های تولید آی‌پی، محاسبه وزن، IPv6
├── ui/
│   └── index.html   # رابط کاربری وب (۳ برگه، دوزبانه، tooltips، جاسازی شده در باینری)
├── xray.exe         # xray-core نسخه v1.8.24 (همراه برنامه)
├── sample.conf      # نمونه کانفیگ WireGuard برای تست
├── README.md        # مستندات انگلیسی
├── README.fa.md     # مستندات فارسی
├── BUILD.md         # راهنمای کامپایل انگلیسی
└── BUILD.fa.md      # راهنمای کامپایل فارسی (این فایل)
```

## معماری

### نحوه عملکرد اسکن

```
کاربر فایل .conf را انتخاب می‌کند  ──>  ParseWarpConfig() کلیدها/آدرس‌ها/بایت‌های Reserved را استخراج می‌کند
                                          │
EndpointGenerator                        │
  └─ IP:port تصادفی                       │
     از پریفیکس‌ها و پورت‌های              │
     شناخته شده کلاودفلر                   ▼
                                    runScan()  ──>  NewScanner()
                                                       │
                                          ┌────────────┼──────────────┐
                                          ▼            ▼              ▼
                                     کارگر ۱       کارگر ۲ ...    کارگر ۱۲
                                          │            │              │
                                     XrayManager  XrayManager    XrayManager
                                     └─ GenerateConfig(endpoint, port)
                                     └─ StartXray() → پردازه xray.exe
                                     └─ WaitForPort() (شنونده SOCKS5)
                                     └─ socks5Handshake() → HTTP GET
                                     └─ StopXray() (کشتن پردازه)
                                          │            │              │
                                          └────────────┴──────────────┘
                                                         │
                                                   مرتب‌سازی بر اساس latency
                                                         │
                                                   برگرداندن نتایج
```

### اجزای اصلی

| فایل | وظیفه |
|------|-------|
| `main.go` | بررسی وجود xray.exe در کنار باینری، راه‌اندازی سرور HTTP روی پورت تصادفی، باز کردن مرورگر |
| `server.go` | اندپوینت‌های API: `/api/scan`, `/api/status/{id}`, `/api/results/{id}`, `/api/stop/{id}`, `/api/apply-endpoint`. جاسازی `ui/` به عنوان فایل‌سیستم. |
| `config.go` | پارس کردن کانفیگ‌های WireGuard. پشتیبانی از هر دو فرمت استاندارد واپ و سبک هاگوارتز. تبدیل همه کلیدها به حروف کوچک، افزودن خودکار `/128` به IPv6 بدون ماسک، استخراج بایت‌های Reserved از فیلدهای S1/S2/S3. |
| `endpoint.go` | تولید اندپوینت‌های تصادفی از پریفیکس‌های آی‌پی شناخته شده واپ کلاودفلر (۱۴ محدوده IPv4، ۴ محدوده IPv6) و ۵۰+ پورت. حذف آی‌پی‌های تکراری. |
| `scanner.go` | ۱۲ کارگر هم‌زمان. هر کارگر: کانفیگ ایکس‌ری می‌سازد → ایکس‌ری را اجرا می‌کند → منتظر SOCKS5 می‌ماند → دست دادن SOCKS5 انجام می‌دهد → HTTP GET به gstatic.com → بررسی ۲۰۴ → ثبت latency. |
| `xray.go` | ساخت کانفیگ JSON ایکس‌ری با خروجی WireGuard، ورودی SOCKS5، قوانین مسیریابی. مدیریت چرخه حیات پردازه (اجرا، انتظار برای پورت، کشتن). |
| `noise.go` | نویز UDP: اندازه تصادفی بسته (۵۰-۱۰۰ بایت) با تأخیر تصادفی (۱-۵ms) که قبل از هر دست دادن WireGuard ارسال می‌شود. برای عبور از مسدودسازی DPI واپ. |
| `proxy.go` | پارس لینک‌های اشتراک VLESS/Trojan (`ParseProxyURL`)، ساخت کانفیگ JSON ایکس‌ری (`BuildXrayJSON`)، تولید لینک اشتراک از کانفیگ (`GenerateShareURL`). همیشه `sni` را در صورت وجود حفظ می‌کند. |
| `cleanip.go` | اسکنر آی‌پی تمیز. تولید آی‌پی از ۲۵ محدوده IPv4 + ۹۱ محدوده IPv6 کلاودفلر (`GenerateIPs`). فاز ۱: پروب TCP با ۵۰۰ کارگر هم‌زمان (`runCleanPhase1TCP`). فاز ۲: اعتبارسنجی xray از طریق SOCKS5 (`validateWithXray`). خروجی کانفیگ (`GenerateExport`). پشتیبانی از حالت SkipPhase2. |
| `replacer.go` | دریافت لینک اشتراک (`FetchSubscription`)، پارس کانفیگ‌های VLESS (`ParseSubscription`)، یکتا کردن با نادیده گرفتن Address/Port/Remark (`DeduplicateConfigs`)، تولید تمام ترکیب‌های کانفیگ×اندپوینت (`GenerateReplacedConfigs`). |
| `cleanip_test.go` | تست‌های Go برای `GenerateIPs`، محاسبه وزن و تولید IPv6. همه قبول. |
| `xray.exe` | xray-core نسخه v1.8.24، همراه در ریپو و زیپ ریلیز |

### API رابط کاربری وب

| اندپوینت | متد | توضیحات |
|----------|------|---------|
| `/` | GET | سرو `index.html` جاسازی شده |
| `/api/scan` | POST | آپلود کانفیگ + پارامترهای JSON ← برگرداندن شناسه job |
| `/api/status/{id}` | GET | برگرداندن `{status, progress, total}` |
| `/api/results/{id}` | GET | برگرداندن اندپوینت‌های working (زنده هنگام اسکن، کامل پس از اتمام) |
| `/api/stop/{id}` | POST | لغو یک اسکن در حال اجرا |
| `/api/apply-endpoint` | POST | آپلود کانفیگ‌ها + اندپوینت ← ذخیره کپی‌های تغییر یافته |
| `/api/clean-scan` | POST | شروع اسکن آی‌پی تمیز ← برگرداندن شناسه job |
| `/api/clean-status/{id}` | GET | برگرداندن وضعیت اسکن تمیز `{status, phase1Progress, phase1Total, phase2Progress, phase2Total}` |
| `/api/clean-results/{id}` | GET | برگرداندن phase1Results + phase2Results (زنده هنگام اسکن، کامل پس از اتمام) |
| `/api/clean-stop/{id}` | POST | توقف یک اسکن تمیز در حال اجرا |
| `/api/clean-export/{id}` | GET | دانلود نتایج اسکن تمیز به صورت فایل متنی |
| `/api/replacer-fetch` | POST | دریافت و یکتا کردن کانفیگ‌ها از لینک اشتراک |
| `/api/replacer-apply` | POST | تولید کانفیگ‌های جایگزین شده از کانفیگ‌های انتخاب شده + اندپوینت‌ها |

### جزئیات پارس کانفیگ

پارسر دو فرمت کانفیگ را پشتیبانی می‌کند:

**استاندارد واپ:**
```ini
[Interface]
PrivateKey = ...
Address = 2606:4700:110:8d48:...
Reserved = 0,0,0
MTU = 1280

[Peer]
PublicKey = bmXOC+F1FxEMF9dyiK2H5/1SUtzH0JuVo51h2wPfgyo=
Endpoint = 162.159.192.1:2408
```

**سبک هاگوارتز (فیلدهای اضافی):**
```ini
[Interface]
PrivateKey = ...
Address = ...
S1 = 0
S2 = 0
S3 = 0
Jc = ...
Jmin = ...
H1 = ...
H2 = ...
H3 = ...
H4 = ...
I1 = ...
I2 = ...

[Peer]
PublicKey = ...
Endpoint = ...
```

پارسر همه کلیدها را به حروف کوچک تبدیل می‌کند، فیلدهای ناشناخته را نادیده می‌گیرد، و در صورت نبود فیلد `Reserved` از S1/S2/S3 برای بایت‌های Reserved استفاده می‌کند.

### نویز UDP

وقتی فعال باشد، ایکس‌ری قبل از هر دست دادن WireGuard بسته‌هایی با اندازه تصادفی ارسال می‌کند. این کار باعث می‌شود ترافیک شبیه نویز به نظر برسد به جای یک نشست WireGuard، و به عبور از ISPهایی که واپ را روی پورت استاندارد ۲۴۰۸ مسدود می‌کنند کمک می‌کند.

پیش‌فرض: ۵ بسته نویز، هرکدام ۵۰-۱۰۰ بایت داده تصادفی، با فاصله ۱-۵ms.

## xray-core

xray-core نسخه v1.8.24 همراه در هر فایل ریلیز قرار دارد. برنامه به دنبال `xray.exe` (ویندوز) یا `xray` (لینوکس/مک) در همان پوشه باینری خود می‌گردد. اگر پیدا نشود، پیغام خطا با لینک دانلود نمایش می‌دهد.

### جریان اسکن آی‌پی تمیز

```
کاربر لینک VLESS را وارد می‌کند       CleanIPGenerator
       (اختیاری)                         └─ GenerateIPs()
                                           └─ ۲۵ CIDR IPv4 (۵۹۵۵ زیرشبکه /۲۴)
                                           └─ ۹۱ CIDR IPv6 (توزیع وزنی)
                                                │
                                         فاز ۱: پروب TCP
                                           └─ ۵۰۰ کارگر هم‌زمان
                                           └─ net.DialTimeout(ip:port, 2s)
                                           └─ نوشتن نتایج به صورت تدریجی در job.Phase1Progress
                                                │
                                   ┌───────────┴──────────┐
                                   ▼                      ▼
                             SkipPhase2=true         SkipPhase2=false
                                   │                      │
                              اتمام (status)       فاز ۲: اعتبارسنجی xray
                                                   └─ مرتب‌سازی فاز ۱ بر اساس latency
                                                   └─ انتخاب N تا از بهترین‌ها (تعداد فاز ۲)
                                                   └─ ۱۲ کارگر xray هم‌زمان
                                                   └─ هرکدام: BuildXrayJSON → StartXray → SOCKS5 → HTTP GET
                                                   └─ نوشتن نتایج تدریجی در job.Phase2Progress
                                                        │
                                                   GenerateExport()
                                                   └─ لینک‌های VLESS
                                                   └─ لیست خام IP:port
```

### جریان جایگزین آی‌پی

```
لینک اشتراک ──> FetchSubscription()
                  └─ HTTP GET → base64.decode → ParseSubscription()
                       │
                  DeduplicateConfigs()
                  └─ اثر انگشت: حذف Address, Port, Remark
                  └─ برگرداندن قالب‌های کانفیگ یکتا
                       │
                  کاربر کانفیگ‌ها را انتخاب می‌کند + اندپوینت‌ها را می‌چسباند
                       │
                  GenerateReplacedConfigs(configs, endpoints)
                  └─ ضرب دکارتی: هر کانفیگ × هر اندپوینت
                  └─ رد کردن موارد تکراری (همان کانفیگ + همان اندپوینت)
                  └─ افزودن " @ endpoint" به هر remark
                  └─ برگرداندن لینک‌های VLESS
```

## متغیرهای محیطی

هیچ متغیر محیطی لازم نیست. همه چیز از طریق رابط کاربری وب در زمان اجرا تنظیم می‌شود.

## نکات مختص پلتفرم

### لینوکس

پس از extract، فایل xray را قابل اجرا کنید: `chmod +x xray`. برنامه مرورگر را از طریق `xdg-open` باز می‌کند — در صورت نیاز `xdg-utils` را نصب کنید:

| توزیع | دستور |
|---|---|
| دبیان / اوبونتو / مینت | `sudo apt install xdg-utils` |
| فدورا / RHEL / سنت‌اواس | `sudo dnf install xdg-utils` |
| آرچ / مانجارو | `sudo pacman -S xdg-utils` |

### Termux / اندروید

فایل ریلیز `termux-arm64` شامل نسخه اندروید xray-core و باینری `linux/arm64` برنامه است.

1. فایل `Cloudflare-Scanner-*-termux-arm64.tar.gz` رادانلود و extract کنید:
   ```bash
   tar -xzf Cloudflare-Scanner-*-termux-arm64.tar.gz
   ```
2. هر دو فایل را قابل اجرا کنید: `chmod +x Cloudflare-Scanner xray`
3. برنامه به طور خودکار Termux را از طریق `$PREFIX` تشخیص می‌دهد و مرورگر را با `termux-open-url` باز می‌کند
4. اجرا: `./Cloudflare-Scanner`

> در صورت نیاز نصب `termux-open-url`: `pkg install termux-open-url`

### مک
- پس از extract، فایل xray را قابل اجرا کنید: `chmod +x xray`
- برنامه مرورگر را از طریق `open` باز می‌کند (داخلی)
- اگر مک باینری امضا نشده را بلاک کرد، اجرا کنید: `xattr -d com.apple.quarantine Cloudflare-Scanner`

## مشارکت

1. پروژه را Fork کنید
2. یک شاخه (branch) ایجاد کنید (`git checkout -b my-thing`)
3. تغییرات را اعمال کنید، با `go vet ./...` بررسی کنید
4. کامپایل و تست کنید: `go build -o Cloudflare-Scanner.exe . && .\Cloudflare-Scanner.exe`
5. Commit با پیام واضح
6. Push کنید و PR باز کنید

## لایسنس

MIT
