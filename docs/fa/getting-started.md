# شروع کار

## ۱. دانلود

به [صفحه ریلیزها](https://github.com/QMahyar/Cloudflare-Scanner/releases) بروید و فایل مخصوص پلتفرم خود را دانلود کنید:

| پلتفرم | فایل مورد نظر |
|---|---|
| ویندوز (Intel/AMD) | `Cloudflare-Scanner-*-windows-amd64.tar.gz` |
| ویندوز (ARM, مثل Surface Pro X) | `Cloudflare-Scanner-*-windows-arm64.tar.gz` |
| لینوکس (Intel/AMD) | `Cloudflare-Scanner-*-linux-amd64.tar.gz` |
| لینوکس (ARM, مثل Raspberry Pi) | `Cloudflare-Scanner-*-linux-arm64.tar.gz` |
| مک (Intel) | `Cloudflare-Scanner-*-darwin-amd64.tar.gz` |
| مک (Apple Silicon M1/M2/M3) | `Cloudflare-Scanner-*-darwin-arm64.tar.gz` |
| Termux / اندروید (ARM64) | `Cloudflare-Scanner-*-termux-arm64.tar.gz` |

## ۲. extract و اجرا

### ویندوز

1. روی فایل `.tar.gz` راست کلیک کنید و **Extract All** را انتخاب کنید (یا با `tar.exe -xzf` extract کنید)
2. پوشه extract شده را باز کنید
3. روی `Cloudflare-Scanner.exe` دوبار کلیک کنید

### لینوکس / مک

```bash
# ترمینال را در پوشه Downloads باز کنید
tar -xzf Cloudflare-Scanner-*-linux-amd64.tar.gz
cd Cloudflare-Scanner-*-linux-amd64
chmod +x Cloudflare-Scanner xray
./Cloudflare-Scanner
```

**نکته:** لینوکس برای باز شدن خودکار مرورگر به `xdg-utils` نیاز دارد:

- Debian/Ubuntu: `sudo apt install xdg-utils`
- Fedora: `sudo dnf install xdg-utils`
- Arch: `sudo pacman -S xdg-utils`

### Termux (اندروید)

```bash
curl -sL https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/termux-setup.sh | sh
```

این دستور آخرین ریلیز Termux را دانلود می‌کند، extract می‌کند و یک دستور `scan` می‌سازد. بعداً فقط کافی است تایپ کنید `scan`.

نیازی به بسته اضافی نیست — برنامه از `termux-open-url` برای باز کردن مرورگر استفاده می‌کند.

## ۳. باز کردن رابط کاربری وب

یک برگه مرورگر به صورت خودکار در `http://127.0.0.1:XXXXX` باز می‌شود (پورت هر بار تغییر می‌کند).

اگر مرورگر باز نشد، خروجی ترمینال را نگاه کنید. آدرس شبیه این است:

```
Web UI: http://127.0.0.1:53671
```

## ۴. تغییر زبان

روی دکمه **English** در گوشه بالا-راست کلیک کنید تا بین انگلیسی و فارسی جابجا شوید. تمام اجزای رابط کاربری (برچسب‌ها، placeholders، tooltips) بلافاصله تغییر می‌کنند.

## ۵. بستن برنامه

پنجره ترمینال را ببندید، یا در ترمینال **Ctrl+C** بزنید.

---

## محتویات فایل فشرده

| فایل | توضیح |
|---|---|
| `Cloudflare-Scanner` (یا `.exe`) | برنامه اصلی — یک سرور وب + موتور اسکن |
| `xray` (یا `xray.exe`) | xray-core نسخه v1.8.24 — اعتبارسنجی اندپوینت و اتصالات پروکسی |

هر دو فایل لازم هستند. آنها را در یک پوشه نگه دارید.

---

## گردش کار اولیه

یک جلسه معمولی اولیه:

1. **دریافت کانفیگ واپ** — از ژنراتورهای آنلاین در بخش راهنمای برنامه استفاده کنید (در برگه Endpoint Scanner به پایین اسکرول کنید)
2. **اسکن اندپوینت‌های واپ** — فایل `.conf` خود را آپلود کنید، تعداد اسکن را تنظیم کنید و اسکن کنید
3. **اعمال اندپوینت خوب** — سریع‌ترین نتیجه را انتخاب کنید، فایل‌های کانفیگ خود را انتخاب کنید، کپی‌های تغییریافته را ذخیره کنید
4. **(اختیاری) اسکن آی‌پی تمیز** — یک لینک VLESS بچسبانید، آی‌پی‌های پروکسی working کلاودفلر را پیدا کنید
5. **(اختیاری) جایگزینی آی‌پی در کانفیگ‌ها** — اشتراک یا کانفیگ‌ها + اندپوینت‌ها را بچسبانید، کانفیگ‌های تازه تولید کنید
