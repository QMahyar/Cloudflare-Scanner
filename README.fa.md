# Cloudflare Scanner

**ابزار سه‌دریکراس‌پلتفرم** — اسکن اندپوینت‌های واپ، پیدا کردن آی‌پی‌های پروکسی تمیز کلاودفلر، و جایگزینی IP:port در کانفیگ‌های اشتراک.

[![آخرین نسخه](https://img.shields.io/github/v/release/QMahyar/Cloudflare-Scanner?label=version&style=flat-square)](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest)
[![دانلودها](https://img.shields.io/github/downloads/QMahyar/Cloudflare-Scanner/total?style=flat-square)](https://github.com/QMahyar/Cloudflare-Scanner/releases)
[![لایسنس](https://img.shields.io/github/license/QMahyar/Cloudflare-Scanner?style=flat-square)](LICENSE)

> ## 🌐 English Version
>
> [**View English README.md**](README.md) — Complete English documentation.
>
> [All English docs](docs/index.md)

---

## دانلود

| پلتفرم | معماری | دانلود |
|--------|--------|--------|
| 🪟 ویندوز | amd64 | [`Cloudflare-Scanner-*-windows-amd64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 🪟 ویندوز | arm64 | [`Cloudflare-Scanner-*-windows-arm64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 🐧 لینوکس | amd64 | [`Cloudflare-Scanner-*-linux-amd64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 🐧 لینوکس | arm64 | [`Cloudflare-Scanner-*-linux-arm64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 🍎 مک (Intel) | amd64 | [`Cloudflare-Scanner-*-darwin-amd64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 🍎 مک (Apple Silicon) | arm64 | [`Cloudflare-Scanner-*-darwin-arm64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |
| 📱 Termux / اندروید | arm64 | [`Cloudflare-Scanner-*-termux-arm64.tar.gz`](https://github.com/QMahyar/Cloudflare-Scanner/releases/latest) |

همراه با **xray-core v1.8.24** — نیاز به دانلود جداگانه نیست.

---

## شروع سریع

| پلتفرم | دستور |
|--------|-------|
| **ویندوز** | فایل `.tar.gz` را extract کنید، روی `Cloudflare-Scanner.exe` دوبار کلیک کنید |
| **لینوکس / مک** | `tar -xzf *.tar.gz && chmod +x Cloudflare-Scanner xray && ./Cloudflare-Scanner` |
| **Termux** | `curl -sL https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/termux-setup.sh \| sh` سپس `scan` |

۱. یک برگه مرورگر در `http://127.0.0.1:XXXXX` باز می‌شود
۲. فایل `.conf` واپ را آپلود کنید → **Start Scan** → سریع‌ترین اندپوینت را انتخاب کنید
۳. **Ctrl+C** برای بستن

---

## راهنمای سیستم‌عامل‌ها

<details>
<summary><strong>ویندوز</strong> — extract، اجرا، فایروال</summary>

- **Extract**: روی `.tar.gz` راست کلیک → **Extract All** یا `tar.exe -xzf Cloudflare-Scanner-*-windows-amd64.tar.gz`
- **اجرا**: روی `Cloudflare-Scanner.exe` دوبار کلیک کنید
- **فایروال**: **Allow** را بزنید
- **عیب‌یابی**: آنتی‌ویروس ممکن است `xray.exe` را بلاک کند — استثنا بگذارید. با Administrator اجرا کنید.
</details>

<details>
<summary><strong>لینوکس</strong> — پیش‌نیازها، extract، اجرا</summary>

```bash
# پیش‌نیازها
sudo apt install xdg-utils tar   # Debian/Ubuntu
sudo dnf install xdg-utils tar   # Fedora
sudo pacman -S xdg-utils tar     # Arch

# Extract و اجرا
tar -xzf Cloudflare-Scanner-*-linux-amd64.tar.gz
cd Cloudflare-Scanner-*-linux-amd64
chmod +x Cloudflare-Scanner xray
./Cloudflare-Scanner
```

**ARM/ARM64** (Raspberry Pi): از `*-linux-arm64.tar.gz` استفاده کنید.
</details>

<details>
<summary><strong>مک</strong> — extract، اجرا، Gatekeeper</summary>

```bash
tar -xzf Cloudflare-Scanner-*-darwin-amd64.tar.gz
cd Cloudflare-Scanner-*-darwin-amd64
chmod +x Cloudflare-Scanner xray
./Cloudflare-Scanner
```

**Apple Silicon**: از `*-darwin-arm64.tar.gz` استفاده کنید.

**Gatekeeper**: اگر مسدود شد، به **System Settings → Privacy & Security → Open Anyway** بروید یا:
```bash
xattr -d com.apple.quarantine xray
```
</details>

<details>
<summary><strong>Termux / اندروید</strong> — نصب یک خطی</summary>

```bash
curl -sL https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/termux-setup.sh | sh
scan
```

- بسته اضافی نیاز نیست
- دستور `scan` بعد از بستن Termux هم می‌ماند
- به‌روزرسانی: دوباره دستور یک خطی را اجرا کنید
- حذف: `rm -rf ~/cloudflare-scanner && sed -i '/alias scan=/d' ~/.bashrc`
</details>

---

## سه ابزار

| ابزار | عملکرد | زمان استفاده |
|-------|--------|-------------|
| **اسکنر اندپوینت** | اندپوینت‌های واپ WireGuard را با فایل `.conf` تست می‌کند | به اندپوینت واپ working نیاز دارید |
| **اسکنر آی‌پی (تمیز)** | محدوده آی‌پی‌های کلاودفلر را برای پروکسی‌های تمیز با VLESS/Trojan اسکن می‌کند | به آی‌پی‌های تمیز برای v2ray نیاز دارید |
| **جایگزین آی‌پی** | IP:port را در کانفیگ‌های اشتراک با اندپوینت‌های تمیز جایگزین می‌کند | کانفیگ‌های قدیمی و آی‌پی‌های تازه دارید |

---

## ویژگی‌ها

- **سه ابزار یکپارچه** — اسکنر اندپوینت، اسکنر آی‌پی، جایگزین آی‌پی
- **نویز UDP** — padding تصادفی + تأخیر برای عبور از مسدودسازی DPI واپ
- **نتایج زنده** — اندپوینت‌ها هم‌زمان با قبول شدن هر تست نمایش داده می‌شوند
- **اسکن آی‌پی تمیز** — از ۲۵ محدوده IPv4 + ۹۱ محدوده IPv6 کلاودفلر
- **پشتیبانی از اشتراک** — دریافت، یکتا کردن و جایگزینی IP:port در کانفیگ‌های VLESS/Trojan
- **اعمال دسته‌جمعی** — یک اندپوینت را به چندین فایل کانفیگ هم‌زمان اعمال کنید
- **رابط کاربری سازگار با موبایل** — واکنش‌گرا، دکمه‌های لمسی با سایز مناسب
- **انتخاب پوشه** — دیالوگ بومی سیستم‌عامل برای انتخاب پوشه خروجی (کرومیوم)
- **رابط وب دو زبانه** — انگلیسی و فارسی با جابجایی فوری
- **خودکفا** — شامل xray-core v1.8.24، نیاز به نصب ندارد
- **قابل لغو** — توقف، ادامه یا ریست اسکن در هر زمان

---

## تغییرات نسخه‌ها

### v1.8.0 (نسخه فعلی)
- رابط کاربری واکنش‌گرا برای موبایل — تب‌ها، جدول‌ها، دکمه‌ها در صفحه‌های ۳۶۰px به بالا
- دکمه انتخاب پوشه خروجی (با `showDirectoryPicker` در کرومیوم)
- نمایش هر کانفیگ با دکمه کپی، QR و textarea قابل انتخاب
- Page design برای صفحه‌های کوچک
- رفع مشکل صفحه‌آرایی RTL فارسی در موبایل

[تغییرات کامل ←](https://github.com/QMahyar/Cloudflare-Scanner/releases)

---

## دریافت کانفیگ واپ

برنامه شامل یک بخش راهنمای کامل با لینک‌های ژنراتورهای آنلاین، بات‌ها و کانال‌های تلگرام، ابزارهای خط فرمان و اپلیکیشن‌های کلاینت است. در برگه **Endpoint Scanner** به پایین اسکرول کنید.

---

## کامپایل از سورس

نیاز به **Go 1.26+** دارد (کامپایلر C لازم نیست). برای راهنمای کامل به [BUILD.fa.md](BUILD.fa.md) مراجعه کنید.

---

## مستندات

| راهنما | توضیح |
|--------|-------|
| [شروع کار](docs/fa/getting-started.md) | تنظیمات اولیه و گردش کار |
| [اسکنر اندپوینت](docs/fa/endpoint-scanner.md) | راهنمای کامل اسکن اندپوینت واپ |
| [اسکنر آی‌پی](docs/fa/ip-scanner.md) | اسکن دو فازه آی‌پی تمیز |
| [جایگزین آی‌پی](docs/fa/ip-replacer.md) | جایگزینی دسته‌جمعی آی‌پی |
| [سوالات متداول](docs/fa/faq.md) | عیب‌یابی و سوالات رایج |
| [BUILD.fa.md](BUILD.fa.md) | کامپایل از سورس، معماری، API |

---

## لایسنس

MIT
