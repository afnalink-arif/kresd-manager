package api

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"os"
	"path/filepath"
)

type BlockPageConfig struct {
	Title       string `json:"title"`
	Subtitle    string `json:"subtitle"`
	Message     string `json:"message"`
	Contact     string `json:"contact"`
	BgColor     string `json:"bg_color"`
	AccentColor string `json:"accent_color"`
	ShowDomain  bool   `json:"show_domain"`
	ShowLogo    bool   `json:"show_logo"`
	FooterText  string `json:"footer_text"`
}

var defaultBlockPageConfig = BlockPageConfig{
	Title:       "Situs Ini Tidak Dapat Diakses",
	Subtitle:    "Berdasarkan kebijakan Kementerian Komunikasi dan Digital (Komdigi) Republik Indonesia, akses ke situs ini telah dibatasi demi perlindungan pengguna internet Indonesia.",
	Message:     "Pemblokiran ini merupakan bagian dari upaya menciptakan ruang digital yang aman, sehat, dan bertanggung jawab bagi seluruh masyarakat. Terima kasih atas pengertian dan kerja sama Anda.",
	Contact:     "",
	BgColor:     "#0a0e1a",
	AccentColor: "#dc2626",
	ShowDomain:  true,
	ShowLogo:    true,
	FooterText:  "Internet Sehat dan Aman — Kementerian Komunikasi dan Digital RI",
}

func (s *Server) getBlockPageConfig() BlockPageConfig {
	cfg := defaultBlockPageConfig

	ctx := context.Background()
	s.pg.QueryRow(ctx,
		`SELECT title, subtitle, message, contact, bg_color, accent_color, show_domain, show_logo, footer_text
		 FROM blockpage_config WHERE id = 1`,
	).Scan(&cfg.Title, &cfg.Subtitle, &cfg.Message, &cfg.Contact, &cfg.BgColor, &cfg.AccentColor, &cfg.ShowDomain, &cfg.ShowLogo, &cfg.FooterText)

	return cfg
}

func (s *Server) handleGetBlockPageConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.getBlockPageConfig())
}

func (s *Server) handleUpdateBlockPageConfig(w http.ResponseWriter, r *http.Request) {
	var req BlockPageConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	s.pg.Exec(ctx,
		`INSERT INTO blockpage_config (id, title, subtitle, message, contact, bg_color, accent_color, show_domain, show_logo, footer_text, updated_at)
		 VALUES (1, $1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		 ON CONFLICT (id) DO UPDATE SET
		   title=$1, subtitle=$2, message=$3, contact=$4, bg_color=$5,
		   accent_color=$6, show_domain=$7, show_logo=$8, footer_text=$9, updated_at=NOW()`,
		req.Title, req.Subtitle, req.Message, req.Contact, req.BgColor, req.AccentColor, req.ShowDomain, req.ShowLogo, req.FooterText,
	)

	writeJSON(w, map[string]string{"message": "block page config updated"})
}

func (s *Server) handleBlockPageLogo(w http.ResponseWriter, r *http.Request) {
	logoPath := filepath.Join(s.cfg.ProjectDir, "config", "komdigi-logo.webp")
	data, err := os.ReadFile(logoPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/webp")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(data)
}

// handleBlockPage serves the glassmorphism block page HTML
func (s *Server) handleBlockPage(w http.ResponseWriter, r *http.Request) {
	cfg := s.getBlockPageConfig()
	e := html.EscapeString

	domainBlock := ""
	if cfg.ShowDomain {
		domainBlock = `<div class="domain" id="d"></div>`
	}

	logoBlock := ""
	if cfg.ShowLogo {
		logoBlock = fmt.Sprintf(`<div class="logo-section">
<img src="/blockpage/komdigi-logo.webp" alt="Komdigi" class="komdigi-logo" onerror="this.style.display='none'">
<div class="icon" style="background:%s">
<svg fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4.5c-.77-.833-2.694-.833-3.464 0L3.34 16.5c-.77.833.192 2.5 1.732 2.5z"/></svg>
</div>
</div>`, e(cfg.AccentColor))
	}

	contactBlock := ""
	if cfg.Contact != "" {
		contactBlock = fmt.Sprintf(`<div class="contact">%s</div>`, e(cfg.Contact))
	}

	page := fmt.Sprintf(`<!DOCTYPE html>
<html lang="id">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>%s</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{min-height:100vh;display:flex;align-items:center;justify-content:center;background:%s;font-family:'Segoe UI',system-ui,-apple-system,sans-serif;overflow:hidden;color:#e2e8f0}

body::before{content:'';position:fixed;inset:0;background:
  radial-gradient(ellipse at 15%% 50%%,%s12,transparent 65%%),
  radial-gradient(ellipse at 85%% 15%%,rgba(59,130,246,.06),transparent 55%%),
  radial-gradient(ellipse at 50%% 85%%,rgba(99,102,241,.05),transparent 55%%);
  animation:pulse 10s ease-in-out infinite alternate}
@keyframes pulse{0%%{opacity:.5}100%%{opacity:1}}

.orb{position:fixed;border-radius:50%%;filter:blur(100px);opacity:.1;animation:drift 25s ease-in-out infinite}
.orb-1{width:500px;height:500px;background:%s;top:-150px;left:-150px}
.orb-2{width:350px;height:350px;background:#1d4ed8;bottom:-100px;right:-100px;animation-delay:-8s}
.orb-3{width:250px;height:250px;background:#7c3aed;top:40%%;right:20%%;animation-delay:-16s}
@keyframes drift{0%%,100%%{transform:translate(0,0) scale(1)}33%%{transform:translate(40px,-50px) scale(1.08)}66%%{transform:translate(-30px,40px) scale(.94)}}

.card{position:relative;z-index:1;max-width:520px;width:92%%;text-align:center;padding:2.5rem 2rem 2rem;
  background:rgba(255,255,255,.03);
  backdrop-filter:blur(24px);-webkit-backdrop-filter:blur(24px);
  border-radius:1.5rem;border:1px solid rgba(255,255,255,.06);
  box-shadow:0 8px 40px rgba(0,0,0,.4),0 0 0 1px rgba(255,255,255,.02) inset}
.card::before{content:'';position:absolute;inset:0;border-radius:1.5rem;background:linear-gradient(160deg,rgba(255,255,255,.05) 0%%,transparent 40%%);pointer-events:none}

.logo-section{display:flex;align-items:center;justify-content:center;gap:.75rem;margin-bottom:1.5rem}
.komdigi-logo{height:48px;width:auto;filter:brightness(0) invert(1);opacity:.85}
.icon{width:44px;height:44px;border-radius:50%%;display:flex;align-items:center;justify-content:center;box-shadow:0 0 25px %s30;flex-shrink:0}
.icon svg{width:22px;height:22px;color:#fff}

h1{font-size:1.15rem;font-weight:700;color:#f8fafc;letter-spacing:-.01em;margin-bottom:.75rem;line-height:1.4}

.domain{font-size:.78rem;color:%s;font-family:'SF Mono',Monaco,Consolas,monospace;background:rgba(0,0,0,.35);padding:.55rem 1rem;border-radius:.65rem;margin:1rem auto;word-break:break-all;border:1px solid %s15;max-width:90%%;display:inline-block}

.desc{font-size:.8rem;color:#94a3b8;line-height:1.75;margin-bottom:.5rem;text-align:justify;text-align-last:center}
.desc-secondary{font-size:.75rem;color:#64748b;line-height:1.7;text-align:justify;text-align-last:center}

.divider{width:40px;height:2px;background:linear-gradient(90deg,%s,%s00);margin:1rem auto;border-radius:1px}

.contact{margin-top:1rem;padding:.6rem 1rem;background:rgba(59,130,246,.06);border:1px solid rgba(59,130,246,.1);border-radius:.6rem;font-size:.72rem;color:#93c5fd}

.footer{margin-top:1.5rem;font-size:.63rem;color:#475569;letter-spacing:.02em;line-height:1.5}

.badge{display:inline-flex;align-items:center;gap:.35rem;margin-top:1rem;padding:.4rem .8rem;background:rgba(34,197,94,.06);border:1px solid rgba(34,197,94,.12);border-radius:2rem;font-size:.65rem;color:#4ade80}
.badge svg{width:12px;height:12px}
</style>
</head>
<body>
<div class="orb orb-1"></div>
<div class="orb orb-2"></div>
<div class="orb orb-3"></div>
<div class="card">
%s
<h1>%s</h1>
%s
<p class="desc">%s</p>
<div class="divider"></div>
<p class="desc-secondary">%s</p>
%s
<div class="badge">
<svg fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"/></svg>
Dilindungi DNS Filtering
</div>
<div class="footer">%s</div>
</div>
<script>var d=document.getElementById('d');if(d)d.textContent=location.hostname</script>
</body>
</html>`,
		e(cfg.Title),
		e(cfg.BgColor),
		e(cfg.AccentColor),
		e(cfg.AccentColor),
		e(cfg.AccentColor),
		e(cfg.AccentColor),
		e(cfg.AccentColor),
		e(cfg.AccentColor),
		e(cfg.AccentColor),
		logoBlock,
		e(cfg.Title),
		domainBlock,
		e(cfg.Subtitle),
		e(cfg.Message),
		contactBlock,
		e(cfg.FooterText),
	)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(page))
}
