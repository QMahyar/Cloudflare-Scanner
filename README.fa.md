# Cloudflare Scanner

**یافتن اندپوینت‌های Warp و آی‌پی‌های تمیز Cloudflare — سریع، رایگان، بدون نیاز به نصب.**

[![آخرین نسخه](https://img.shields.io/github/v/release/QMahyar/Cloudflare-Scanner?style=flat-square&label=دانلود)](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest)
[![CI](https://img.shields.io/github/actions/workflow/status/QMahyar/Cloudflare-Scanner/ci.yml?branch=master&style=flat-square&label=CI)](https://github.com/QMahyar/Cloudflare-Scanner/actions/workflows/ci.yml)
[![دانلودها](https://img.shields.io/github/downloads/QMahyar/Cloudflare-Scanner/total?style=flat-square)](https://github.com/QMahyar/Cloudflare-Scanner/releases)
[![لایسنس](https://img.shields.io/github/license/QMahyar/Cloudflare-Scanner?style=flat-square)](LICENSE)

> ### 🌐 English Version
> [**View English README →**](README.md)

---

## این ابزار چیست؟

**Cloudflare Scanner** یک ابزار دسکتاپ کراس‌پلتفرم است که به شما کمک می‌کند:

- **اندپوینت‌های Warp working پیدا کنید** — صدها IP:port از سرورهای Cloudflare WARP/WireGuard را اسکن کرده و بر اساس تأخیر رتبه‌بندی می‌کند تا سریع‌ترین را انتخاب کنید.
- **آی‌پی‌های تمیز پیدا کنید** — محدوده‌های IP کلاودفلر را از طریق TCP پروب کرده و سپس با xray-core (VLESS/Trojan) اعتبارسنجی می‌کند.
- **کانفیگ‌ها را یکجا به‌روز کنید** — IP:port را در هر تعداد کانفیگ اشتراک با یک کلیک جایگزین کنید.

یک وب‌سرور کوچک محلی اجرا می‌کند و یک تب مرورگر باز می‌کند — بدون نصب، بدون پیش‌نیاز.

### چه کسانی به این ابزار نیاز دارند؟

اگر از **Cloudflare Warp**، **v2ray**، **Nekobox**، **Sing-box** یا هر کلاینت پروکسی مبتنی بر شبکه Cloudflare استفاده می‌کنید، عملکرد شما کاملاً به IP:port انتخابی بستگی دارد. ISPها اغلب اندپوینت‌های خاصی را بلاک می‌کنند. این ابزار آن‌هایی را که هنوز کار می‌کنند پیدا کرده و بر اساس سرعت رتبه‌بندی می‌کند.

---

## دانلود

| پلتفرم | معماری | دانلود |
|--------|--------|--------|
| 🪟 ویندوز | x86-64 | [`windows-amd64.zip`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 🪟 ویندوز | ARM64 | [`windows-arm64.zip`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 🐧 لینوکس | x86-64 | [`linux-amd64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 🐧 لینوکس | ARM64 / Raspberry Pi | [`linux-arm64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 🍎 مک | Intel | [`darwin-amd64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 🍎 مک | Apple Silicon | [`darwin-arm64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 📱 اندروید (Termux) | ARM64 | [`termux-arm64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |

هر فایل شامل برنامه **و** xray-core است — نیازی به نصب چیز دیگری نیست.

---

## نصب یک‌خطی

```powershell
# ویندوز — PowerShell را به عنوان Administrator اجرا کنید
irm https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/install-windows.ps1 | iex
```

```sh
# لینوکس
curl -fsSL https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/install-linux.sh | sh

# مک
curl -fsSL https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/install-macos.sh | sh

# Termux / اندروید
curl -fsSL https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/termux-setup.sh | sh
```

اسکریپت‌ها به‌صورت خودکار معماری CPU را تشخیص داده، نسخه صحیح را دانلود کرده و `cloudflare-scanner` (یا `scan` در Termux) را به PATH اضافه می‌کنند.

---

## نصب دستی

<details>
<summary><strong>ویندوز</strong></summary>

۱. فایل `Cloudflare-Scanner-*-windows-amd64.zip` را از [Releases](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) دانلود کنید  
۲. راست‌کلیک → **Extract All** (یا با [7-Zip](https://7-zip.org))  
۳. روی `Cloudflare-Scanner.exe` دوبار کلیک کنید  
۴. یک تب مرورگر به صورت خودکار باز می‌شود  

**عیب‌یابی:** آنتی‌ویروس ممکن است `xray.exe` را بلاک کند — استثنا برای پوشه استخراج‌شده تعریف کنید.
</details>

<details>
<summary><strong>لینوکس</strong></summary>

```bash
tar -xzf Cloudflare-Scanner-*-linux-amd64.tar.gz
chmod +x Cloudflare-Scanner xray
./Cloudflare-Scanner
```

برای ARM64 (Raspberry Pi و غیره): از آرشیو `*-linux-arm64.tar.gz` استفاده کنید.
</details>

<details>
<summary><strong>مک</strong></summary>

```bash
tar -xzf Cloudflare-Scanner-*-darwin-arm64.tar.gz   # Apple Silicon
# یا darwin-amd64 برای Intel
chmod +x Cloudflare-Scanner xray
xattr -dr com.apple.quarantine xray  # حذف flag گیت‌کیپر
./Cloudflare-Scanner
```

اگر مک هنوز برنامه را بلاک کرد: **System Settings → Privacy & Security → Open Anyway**.
</details>

<details>
<summary><strong>Termux / اندروید</strong></summary>

```bash
curl -fsSL https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/termux-setup.sh | sh
scan   # اجرای برنامه
```

- به‌روزرسانی: همان دستور یک‌خطی را دوباره اجرا کنید
- حذف: `rm -rf ~/.local/share/cloudflare-scanner && rm $PREFIX/bin/scan`
</details>

---

## نحوه استفاده

پس از اجرا، یک تب مرورگر در آدرس `http://127.0.0.1:PORT` باز می‌شود. برنامه سه تب دارد:

### تب ۱ — Endpoint Scanner (اسکنر اندپوینت)

برای یافتن اندپوینت سریع Warp WireGuard:

۱. یک فایل `.conf` واپ تهیه کنید — لینک‌ها در داخل برنامه موجودند  
۲. **Use Real Config** را فعال کنید و فایل `.conf` را آپلود کنید  
۳. عمق اسکن را انتخاب کنید (Quick=100 / Normal=500 / Deep=1K+)  
۴. **Start Scan** را بزنید  
۵. نتایج به‌صورت زنده نمایش داده می‌شوند، مرتب‌شده بر اساس تأخیر  
۶. روی یک نتیجه کلیک کنید → آن را به فایل‌های `.conf` اعمال کنید  

> **بدون کانفیگ:** "Use Real Config" را غیرفعال کنید برای اسکن سریع TCP فقط.

### تب ۲ — IP Scanner (اسکنر آی‌پی)

برای یافتن آی‌پی‌های تمیز Cloudflare:

۱. لینک `vless://` یا `trojan://` خود را paste کنید  
۲. **پورت‌های اسکن** را انتخاب کنید: فقط 443، همه HTTPS، همه HTTP+HTTPS، یا انتخاب سفارشی  
۳. عمق اسکن را تنظیم کرده و **Start Clean Scan** بزنید  
۴. **فاز ۱** — پروب TCP سریع روی محدوده IP کلاودفلر  
۵. **فاز ۲** — xray-core بهترین نتایج فاز ۱ را اعتبارسنجی می‌کند  
۶. آی‌پی‌های working را export کنید → در IP Replacer استفاده کنید  

> **انتخاب پورت:** اسکنر تمام ۱۳ پورت رسمی CDN کلاودفلر را پشتیبانی می‌کند (HTTP: 80, 8080, 8880, 2052, 2082, 2086, 2095 · HTTPS: 443, 8443, 2053, 2083, 2087, 2096).

### تب ۳ — IP Replacer (جایگزین آی‌پی)

برای تزریق آی‌پی‌های تازه به کانفیگ‌های موجود:

۱. URL اشتراک **یا** متن raw کانفیگ را paste کنید  
۲. کانفیگ‌هایی که می‌خواهید به‌روز شوند را انتخاب کنید  
۳. اندپوینت‌های `IP:port` یافت‌شده در IP Scanner را paste کنید  
۴. **Generate Configs** را بزنید → Copy All یا Download  

---

## گردش کار کامل (از صفر تا صد)

| مرحله | اقدام | تب |
|-------|-------|-----|
| ۱ | یک فایل `.conf` واپ تهیه کنید | — |
| ۲ | آپلود کانفیگ → Start Scan → سریع‌ترین نتیجه را انتخاب کنید | Endpoint Scanner |
| ۳ | اندپوینت را به فایل‌های `.conf` اعمال کنید | Endpoint Scanner |
| ۴ | لینک `vless://` را paste کنید → اسکن آی‌پی تمیز → export | IP Scanner |
| ۵ | URL اشتراک → انتخاب کانفیگ‌ها → تزریق اندپوینت‌ها | IP Replacer |

> **اولین‌بار؟** مراحل ۱–۳ کافی است برای یک اندپوینت واپ working.

---

## ویژگی‌ها

| ویژگی | جزئیات |
|-------|---------|
| اسکن اندپوینت | تست اندپوینت‌های Warp WireGuard با اعتبارسنجی اختیاری xray |
| اسکن آی‌پی | تولید بر اساس CIDR از ۲۵ محدوده IPv4 + ۹۱ محدوده IPv6 کلاودفلر |
| انتخاب پورت | انتخاب از ۱۳ پورت رسمی CDN کلاودفلر (HTTP + HTTPS) |
| اسکن مجاور | بعد از فاز ۱، محدوده‌های مجاور آی‌پی‌های working را گسترش می‌دهد |
| نویز UDP | padding تصادفی + jitter برای دور زدن بلاک‌های DPI |
| اعمال دسته‌جمعی | به‌روزرسانی تعداد زیادی فایل `.conf` با یک اندپوینت |
| اشتراک | دریافت، یکتاسازی و جایگزینی IP:port در کانفیگ‌های VLESS/Trojan |
| نتایج زنده | اندپوینت‌ها به‌صورت بلادرنگ در حین تأیید نمایش داده می‌شوند |
| رابط دو زبانه | انگلیسی و فارسی، جابجایی فوری |
| رابط موبایل | واکنش‌گرا تا ۳۶۰ پیکسل، بهینه لمسی |
| کد QR | تولید QR برای هر کانفیگ — اسکن با گوشی |
| انتخاب پوشه | دیالوگ بومی سیستم‌عامل برای انتخاب مسیر خروجی |
| خودکفا | همراه xray-core، نیازی به نصب چیز دیگری نیست |
| قابل لغو | توقف، اسکن مجدد یا ریست در هر زمان |

---

## دریافت کانفیگ واپ

تب **Endpoint Scanner** یک بخش راهنمای داخلی دارد با لینک‌هایی به:
- ژنراتورهای آنلاین کانفیگ واپ
- بات‌های تلگرام که کانفیگ تولید می‌کنند
- ابزارهای CLI منبع‌باز
- اپلیکیشن‌های کلاینت WireGuard

به پایین تب Endpoint Scanner اسکرول کنید تا آن‌ها را ببینید.

---

## کامپایل از سورس

نیاز به **Go 1.26+** دارد — بدون نیاز به کامپایلر C.

```bash
git clone https://github.com/QMahyar/Cloudflare-Scanner.git
cd Cloudflare-Scanner
go build -ldflags="-s -w -X 'main.Version=dev'" -o Cloudflare-Scanner .
```

برای راهنمای کامل به [BUILD.fa.md](BUILD.fa.md) مراجعه کنید.

---

## مستندات

| راهنما | توضیح |
|--------|-------|
| [شروع کار](docs/fa/getting-started.md) | تنظیمات اولیه و گردش کار کامل |
| [اسکنر اندپوینت](docs/fa/endpoint-scanner.md) | راهنمای اسکن اندپوینت واپ |
| [اسکنر آی‌پی](docs/fa/ip-scanner.md) | اسکن دو فازه آی‌پی تمیز |
| [جایگزین آی‌پی](docs/fa/ip-replacer.md) | جایگزینی دسته‌جمعی آی‌پی |
| [سوالات متداول](docs/fa/faq.md) | عیب‌یابی و سوالات رایج |
| [BUILD.fa.md](BUILD.fa.md) | کامپایل از سورس، معماری، API |

---

## تاریخچه تغییرات

فایل [CHANGELOG.md](CHANGELOG.md) را ببینید.

---

## لایسنس

[MIT](LICENSE) — استفاده، تغییر و توزیع آزادانه.
