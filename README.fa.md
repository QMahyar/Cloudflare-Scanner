# Cloudflare Scanner

> **یافتن اندپوینت‌های Warp و آی‌پی‌های تمیز Cloudflare — سریع، رایگان، بدون نیاز به نصب.**

[![آخرین نسخه](https://img.shields.io/github/v/release/QMahyar/Cloudflare-Scanner?style=flat-square&label=دانلود)](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest)
[![CI](https://img.shields.io/github/actions/workflow/status/QMahyar/Cloudflare-Scanner/ci.yml?branch=master&style=flat-square&label=CI)](https://github.com/QMahyar/Cloudflare-Scanner/actions/workflows/ci.yml)
[![دانلودها](https://img.shields.io/github/downloads/QMahyar/Cloudflare-Scanner/total?style=flat-square)](https://github.com/QMahyar/Cloudflare-Scanner/releases)
[![لایسنس](https://img.shields.io/github/license/QMahyar/Cloudflare-Scanner?style=flat-square)](LICENSE)

---

> **English** → [README.md](README.md)

---

## نمای کلی

**Cloudflare Scanner** یک ابزار دسکتاپ کراس‌پلتفرم برای پیدا کردن اندپوینت‌های سالم Cloudflare WARP و آی‌پی‌های تمیز پروکسی است، همراه با قابلیت اعمال دسته‌جمعی آن‌ها در فایل‌های کانفیگ. این ابزار [xray-core](https://github.com/XTLS/Xray-core) را برای اعتبارسنجی واقعی همراه خود دارد.

**اگر تازه وارد هستید از اینجا شروع کنید.** دانلود، استخراج، اجرا — یک تب مرورگر باز می‌شود. سه تب تمام کار را انجام می‌دهند.

### این ابزار به چه کسانی کمک می‌کند؟

کسانی که از **Cloudflare WARP**، **v2ray/v2rayN**، **Nekobox**، **Sing-box**، **Clash** یا هر کلاینت پروکسی مبتنی بر شبکه Cloudflare استفاده می‌کنند. ISPها اغلب آی‌پی‌ها و پورت‌های خاصی را مسدود می‌کنند. این ابزار آن‌هایی را که از **شبکه شما** هنوز کار می‌کنند پیدا کرده و بر اساس عملکرد واقعی رتبه‌بندی می‌کند.

---

## شروع سریع

### ۱. دانلود

| پلتفرم | معماری | دانلود |
|--------|--------|--------|
| 🪟 ویندوز | x86-64 | [`windows-amd64.zip`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 🪟 ویندوز | ARM64 | [`windows-arm64.zip`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 🐧 لینوکس | x86-64 | [`linux-amd64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 🐧 لینوکس | ARM64 / Raspberry Pi | [`linux-arm64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 🍎 مک | Intel | [`darwin-amd64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 🍎 مک | Apple Silicon | [`darwin-arm64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 📱 اندروید (Termux) | ARM64 | [`termux-arm64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |

هر فایل شامل برنامه **و** xray-core است.

### ۲. نصب و اجرا

از دستور یک‌خطی مخصوص سیستم‌عامل خودتان استفاده کنید، یا آرشیو را دستی دانلود و استخراج کنید.

<details>
<summary><strong>ویندوز — PowerShell</strong></summary>

```powershell
[Net.ServicePointManager]::SecurityProtocol=[Net.SecurityProtocolType]::Tls12; irm https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/install-windows.ps1 | iex
Cloudflare-Scanner
```

این دستور را در **PowerShell** اجرا کنید، نه Git Bash/WSL. نصب دستی: فایل `.zip` را استخراج کنید، `Cloudflare-Scanner.exe` و `xray.exe` را کنار هم نگه دارید، سپس `Cloudflare-Scanner.exe` را اجرا کنید.

*عیب‌یابی:* آنتی‌ویروس ممکن است `xray.exe` را بلاک کند — برای پوشه استثنا تعریف کنید.
</details>

<details>
<summary><strong>لینوکس</strong></summary>

```bash
curl -fsSL https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/install-linux.sh | sh
cloudflare-scanner
```

نصب دستی:

```bash
tar -xzf Cloudflare-Scanner-*-linux-amd64.tar.gz
chmod +x Cloudflare-Scanner xray
./Cloudflare-Scanner
```

ARM64: از آرشیو `*-linux-arm64.tar.gz` استفاده کنید. این نصب‌کننده را در Git Bash ویندوز اجرا نکنید؛ در ویندوز از دستور PowerShell استفاده کنید.
</details>

<details>
<summary><strong>مک</strong></summary>

```bash
curl -fsSL https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/install-macos.sh | sh
cloudflare-scanner
```

نصب دستی:

```bash
tar -xzf Cloudflare-Scanner-*-darwin-arm64.tar.gz   # Apple Silicon
chmod +x Cloudflare-Scanner xray
xattr -dr com.apple.quarantine Cloudflare-Scanner xray
./Cloudflare-Scanner
```

اگر بلاک شد: **System Settings → Privacy & Security → Open Anyway**.
</details>

<details>
<summary><strong>Termux / اندروید</strong></summary>

```bash
curl -fsSL https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/termux-setup.sh | sh
scan
```

*بروزرسانی:* همان دستور را دوباره اجرا کنید. *حذف:* `rm -rf ~/.local/share/cloudflare-scanner && rm $PREFIX/bin/scan`
</details>

### ۳. استفاده از برنامه

بعد از اجرا، یک مرورگر با سه تب باز می‌شود:

| تب | کاربرد | چه زمانی استفاده کنیم |
|-----|-------------|-------------|
| **Endpoint Scanner** | پیدا کردن اندپوینت‌های WARP | وقتی WARP یا WireGuard شما قطع یا کند است |
| **IP Scanner** | پیدا کردن آی‌پی‌های تمیز Cloudflare | نیاز به آی‌پی تازه برای کانفیگ پروکسی دارید |
| **IP Replacer** | جایگزینی دسته‌جمعی IP:port در کانفیگ‌ها | اندپوینت‌های کارآمد دارید و می‌خواهید به اشتراک‌ها اعمال کنید |

---

## ویژگی‌ها

| ویژگی | جزئیات |
|---------|---------|
| **اسکن اندپوینت WARP** | تست اندپوینت‌های WireGuard با هندشیک بومی WireGuard؛ xray فقط برای اسکن‌های دارای UDP Noise استفاده می‌شود |
| **اسکن آی‌پی تمیز** | دو فاز: پروب TCP سریع → اعتبارسنجی با Xray (VLESS/Trojan) |
| **محدوده‌های دلخواه** | به‌جای مخزن کلاودفلر، CIDR / محدودهٔ `a-b` / تک‌آی‌پی خودتان (یا یک فایل) را اسکن کنید — محدوده‌های کوچک کامل، بزرگ‌ها نمونه‌برداری |
| **انتخاب پورت** | هر ۱۳ پورت رسمی CDN کلاودفلر (HTTP + HTTPS) |
| **نویز UDP** | padding تصادفی + jitter برای دور زدن بلاک DPI |
| **اسکن مجاور** | گسترش در اطراف هر آی‌پی کارآمد برای یافتن اهداف مجاور |
| **جایگزین اشتراک** | دریافت → یکتاسازی → تزریق IP:port تازه در کانفیگ‌ها |
| **اعمال دسته‌جمعی** | بروزرسانی تعداد زیادی فایل `.conf` با یک کلیک |
| **اعتبارسنجی چندمرحله‌ای** | هر اندپوینت چندبار تست می‌شود، میانه تأخیر گزارش می‌شود |
| **تشخیص دیتاسنتر Cloudflare** | مشخص می‌کند کدام دیتاسنتر پاسخ می‌دهد (FRA، AMS، IAD و...) |
| **نتایج زنده** | اندپوینت‌های کارآمد به محض تأیید نمایش داده می‌شوند |
| **رابط دو زبانه** | انگلیسی و فارسی، جابجایی فوری |
| **دوستدار موبایل** | واکنش‌گرا تا ۳۶۰ پیکسل، بهینه لمسی |
| **کد QR** | تولید QR برای هر کانفیگ — اسکن با گوشی |
| **خودکفا** | همراه با xray-core، بدون نیاز به نصب چیز دیگر |
| **قابل لغو** | توقف، اسکن مجدد یا ریست در هر زمان |

---

## گردش کار

### مبتدی — دریافت اندپوینت WARP کارآمد

```
۱. دریافت فایل .conf واپ    ─>  بخش "Getting Warp Configs" داخل برنامه
۲. آپلود → شروع اسکن       ─>  منتظر نتایج بمانید
۳. کلیک روی بهترین نتیجه    ─>  اعمال به فایل‌های .conf
```

### پیشرفته — آی‌پی تمیز + جایگزینی دسته‌جمعی

```
۱. paste لینک vless://      ─>  تب: IP Scanner
۲. شروع اسکن تمیز          ─>  منتظر فاز ۱ + فاز ۲ بمانید
۳. خروجی گرفتن از آی‌پی‌ها
۴. paste URL اشتراک        ─>  تب: IP Replacer
۵. انتخاب کانفیگ‌ها + paste اندپوینت‌ها → Generate
```

---

## امنیت و حریم خصوصی

- وب‌سرور فقط به **127.0.0.1** متصل می‌شود — هیچ دسترسی خارجی ندارد.
- کانفیگ‌ها و اشتراک‌ها **کاملاً روی دستگاه شما** پردازش می‌شوند.
- دریافت اشتراک فقط یک درخواست HTTP به آدرسی که شما تایپ می‌کنید ارسال می‌کند.
- برنامه هیچ گونه تله‌متری، آنالیتیکس یا تماس شبکه‌ای غیر از ترافیک اسکن ندارد.

---

## ساخت از سورس

توسعه‌دهندگان می‌توانند با اسکریپت‌های موجود بدون نیاز به تنظیم دستی Go یا xray محلی ساخت کنند:

```bash
# لینوکس / مک / Termux
./scripts/build.sh           # پلتفرم میزبان
./scripts/build.sh all       # همه ۷ پلتفرم

# ویندوز PowerShell
.\scripts\build.ps1          # پلتفرم میزبان
.\scripts\build.ps1 all      # همه ۷ پلتفرم
```

اسکریپت‌ها Go را در صورت نبود نصب می‌کنند، باینری را کامپایل می‌کنند، xray-core مربوطه را دانلود کرده و آرشیو‌های مشابه release در `dist/` تولید می‌کنند. برای گزینه‌های کامل [BUILD.fa.md](BUILD.fa.md) را ببینید.

---

## مستندات

| راهنما | مخاطب | توضیح |
|--------|-------|-------|
| [شروع کار](docs/fa/getting-started.md) | کاربران | راهنمای اولیه بعد از اجرا |
| [اسکنر اندپوینت](docs/fa/endpoint-scanner.md) | کاربران | آموزش کامل اسکن اندپوینت WARP |
| [اسکنر آی‌پی](docs/fa/ip-scanner.md) | کاربران | آموزش اسکن آی‌پی تمیز |
| [جایگزین آی‌پی](docs/fa/ip-replacer.md) | کاربران | جایگزینی دسته‌جمعی اشتراک |
| [سوالات متداول](docs/fa/faq.md) | کاربران | عیب‌یابی و سوالات رایج |
| [BUILD.fa.md](BUILD.fa.md) | توسعه‌دهندگان | کامپایل، معماری، مرجع API |

---

## تاریخچه تغییرات

[CHANGELOG.md](CHANGELOG.md)

---

## لایسنس

[MIT](LICENSE) — استفاده، تغییر و توزیع آزادانه.
