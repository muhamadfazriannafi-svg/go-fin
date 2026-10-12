# GoFin

![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=white)
![Vite](https://img.shields.io/badge/Vite-8-646CFF?logo=vite&logoColor=white)
![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)
![Supabase](https://img.shields.io/badge/Supabase-Postgres-3ECF8E?logo=supabase&logoColor=white)
![Groq](https://img.shields.io/badge/Groq-LLM-F55036)

Catat keuangan cukup dengan chat ke bot Telegram. Kirim teks bebas (*"makan siang 25rb"*) atau foto struk, GoFin akan memparsing-nya dengan LLM, menyimpannya ke database, lalu menampilkannya sebagai dashboard saldo & riwayat secara real-time.

Proyek ini mengeksplorasi integrasi end-to-end sebuah **AI-powered personal finance tracker**: Telegram Bot API, LLM (teks + vision), Go backend, dan React dashboard dengan Supabase Realtime.

> Ganti baris ini dengan link demo / screenshot: `[Screenshot Dashboard](docs/dashboard.png)`

## Fitur Utama

- **Input natural language** — catat transaksi dengan bahasa sehari-hari, tanpa form.
- **OCR struk via vision LLM** — kirim foto struk, nominal & kategori diekstrak otomatis.
- **Klasifikasi otomatis** — LLM menentukan `type` (income/expense), `category`, dan `description`.
- **Dashboard real-time** — transaksi baru muncul otomatis tanpa refresh (Supabase Realtime).
- **Ringkasan saldo** — total saldo dihitung dari seluruh transaksi.
- **Multi-user by chat ID** — tiap pengguna Telegram dipisahkan lewat `user_id`.

## Arsitektur

```
Telegram  ->  gofin-backend (Go/Gin)  ->  Groq (LLM)  ->  Supabase (Postgres)
                                                                |
                                                          (PostgREST + Realtime)
                                                                v
                                                        gofin-web (React)
```

- **gofin-backend**: webhook Telegram, parsing pesan teks / foto struk via Groq, simpan ke tabel `transactions`.
- **gofin-web**: baca `transactions` dengan anon key, tampilkan saldo & riwayat, auto-refresh via Supabase Realtime.

## Tech Stack

### Frontend (`gofin-web`)

| Bagian | Teknologi | Versi |
| --- | --- | --- |
| Runtime | Node.js | 18+ (disarankan 20/22 LTS) |
| UI | React | ^19.2.8 |
| DOM renderer | react-dom | ^19.2.8 |
| Build tool | Vite | ^8.3.0 |
| Vite plugin | @vitejs/plugin-react (Oxc) | ^6.1.1 |
| Data client | @supabase/supabase-js | ^2.117.3 |
| Lint | ESLint (flat config) | ^10.10.0 |
| Lint plugin | eslint-plugin-react-hooks | ^7.1.1 |
| Lint plugin | eslint-plugin-react-refresh | ^0.5.6 |
| Bahasa | JavaScript (JSX, ESM) | - |

### Backend (`gofin-backend`)

| Bagian | Teknologi | Versi |
| --- | --- | --- |
| Bahasa | Go | 1.27.2 |
| HTTP framework | Gin | v1.12.0 |
| Env loader | godotenv | v1.5.1 |
| LLM (teks & vision) | Groq API (`openai/gpt-oss-20b`, `qwen/qwen3.8-27b`) | - |
| Bot | Telegram Bot API | - |

### Infrastruktur

| Bagian | Teknologi |
| --- | --- |
| Database | Supabase Postgres |
| API data | Supabase PostgREST (auto-generated) |
| Realtime | Supabase Realtime (postgres_changes) |
| Auth | Belum ada (anon key untuk MVP) |

## Struktur Proyek

```
gofin-web/
├── .env                  # kredensial Supabase (tidak di-commit)
├── index.html            # entry HTML, mount #root
├── vite.config.js        # konfigurasi Vite
└── src/
    ├── main.jsx          # bootstrap React (createRoot)
    ├── App.jsx           # dashboard: fetch + realtime + render
    └── index.css         # style global
```

## Setup

### 1. Environment

Buat file `.env` di root `gofin-web/`:

```env
VITE_SUPABASE_URL=https://<project-ref>.supabase.co
VITE_SUPABASE_ANON_KEY=<anon-key>
```

> Hanya variabel ber-prefix `VITE_` yang terekspos ke bundle. **Jangan** taruh `service_role`/secret key di sini karena ikut terkirim ke browser.

### 2. Install & jalankan

```bash
npm install
npm run dev       # dev server (default http://localhost:5173)
npm run build     # build produksi ke dist/
npm run preview   # preview hasil build
npm run lint      # ESLint
```

Vite hanya membaca `.env` saat startup. **Restart** `npm run dev` setiap kali `.env` berubah.

## Skema Data

Tabel `public.transactions` yang dikonsumsi frontend:

| Kolom | Tipe | Keterangan |
| --- | --- | --- |
| `id` | uuid | Primary key |
| `user_id` | text | ID chat Telegram pengirim |
| `type` | text | `income` atau `expense` |
| `amount` | numeric | Nominal |
| `category` | text | Kategori transaksi |
| `description` | text | Deskripsi |
| `transaction_date` | timestamptz | Diisi backend saat insert |

### RLS (penting)

`transactions` mengaktifkan Row Level Security. Tanpa policy, anon key menerima **HTTP 200 dengan array kosong** (`[]`) — bukan error — sehingga dashboard tampak "tidak terkoneksi".

Policy minimal (jalankan di Supabase SQL Editor):

```sql
create policy "anon read transactions"
on public.transactions for select
to anon using (true);
```

> `using (true)` memberi akses baca ke semua baris bagi siapa pun yang memegang anon key. Aman hanya untuk MVP. Untuk produksi: tambahkan Supabase Auth lalu scope ke `user_id`.

### Realtime

Agar auto-refresh saat ada insert baru, tambahkan tabel ke publication:

```sql
alter publication supabase_realtime add table public.transactions;
```

Realtime tetap menghormati RLS, jadi policy SELECT anon harus ada.

## Alur Data di `App.jsx`

1. `createClient(VITE_SUPABASE_URL, VITE_SUPABASE_ANON_KEY)` (`App.jsx:7`).
2. `useEffect` -> `fetchTransactions()` memanggil `from('transactions').select('*').order('transaction_date', { ascending: false })`.
3. Saldo dihitung di klien: `income` menambah, `expense` mengurangi (`App.jsx:37`).
4. Berlangganan channel `realtime-transactions`, event `INSERT` memicu re-fetch (`App.jsx:17`). Channel dibersihkan saat unmount.

## Cara Kerja

1. Pengguna mengirim pesan teks atau foto struk ke bot Telegram.
2. Telegram meneruskan update ke webhook `POST /webhook/telegram` di `gofin-backend`.
3. Backend memilih jalur: teks -> `openai/gpt-oss-20b`, foto -> `qwen/qwen3.8-27b` (vision). Keduanya diminta mengembalikan JSON terstruktur (`amount`, `type`, `category`, `description`).
4. Hasil validasi disimpan ke tabel `transactions` via Supabase REST memakai service key.
5. Bot membalas ringkasan transaksi ke pengguna.
6. Insert baru memicu event Supabase Realtime -> dashboard React memperbarui saldo & riwayat otomatis.

## Highlight Teknis

- **LLM structured output** — memaksa JSON murni (`response_format: json_object`) agar hasil parsing bisa langsung di-unmarshal, bukan teks bebas.
- **Vision fallback** — karena model vision terbatas di tier gratis Groq, model dipilih dari daftar `input_modalities` yang benar-benar tersedia (`qwen/qwen3.8-27b`).
- **Realtime subscription** — `postgres_changes` + cleanup channel pada unmount untuk menghindari memory leak.
- **Proteksi data** — `service_role` hanya ada di backend; frontend memakai anon key + RLS.
- **Guard input** — pesan non-transaksi (mis. `/start`) tidak ikut tersimpan sebagai baris kosong.

## Roadmap

- [ ] Auth Supabase + scope RLS ke `user_id` (ganti policy `using (true)`).
- [ ] Filter periode & kategori di dashboard.
- [ ] Kategori kustom / budget bulanan.
- [ ] Grafik tren pengeluaran.
- [ ] Deployment: backend (Fly.io/Render) + web (Vercel/Netlify).

## Troubleshooting

| Gejala | Penyebab | Solusi |
| --- | --- | --- |
| Saldo Rp 0, list kosong, tanpa error | RLS aktif tanpa policy anon | Tambah policy SELECT (lihat RLS) |
| Data tidak auto-refresh | Tabel belum masuk publication realtime | `alter publication supabase_realtime add table ...` |
| Perubahan `.env` tidak terbaca | Vite membaca env saat startup | Restart dev server |
| `401`/`invalid api key` | URL/key salah atau key tertukar | Cocokkan dengan project Supabase |

Verifikasi cepat bahwa anon key boleh baca:

```bash
curl "https://<project-ref>.supabase.co/rest/v1/transactions?select=*" \
  -H "apikey: <anon-key>"
```

Hasil seharusnya array berisi data, bukan `[]`.
