package api

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
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

func (s *Server) getBlockPageConfig() BlockPageConfig {
	cfg := BlockPageConfig{
		Title:       "Akses Diblokir",
		Subtitle:    "Domain ini telah diblokir oleh administrator jaringan melalui DNS filtering.",
		Message:     "Jika Anda merasa ini adalah kesalahan, silakan hubungi administrator.",
		BgColor:     "#0f172a",
		AccentColor: "#ef4444",
		ShowDomain:  true,
		ShowLogo:    true,
		FooterText:  "DNS Filter — Knot DNS Monitor",
	}

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
		logoBlock = fmt.Sprintf(`<div class="icon" style="background:%s">
<svg fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728A9 9 0 015.636 5.636m12.728 12.728L5.636 5.636"/></svg>
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
body{min-height:100vh;display:flex;align-items:center;justify-content:center;background:%s;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;overflow:hidden}

/* animated gradient background */
body::before{content:'';position:fixed;inset:0;background:radial-gradient(ellipse at 20%% 50%%,%s15,transparent 70%%),radial-gradient(ellipse at 80%% 20%%,rgba(59,130,246,.08),transparent 60%%),radial-gradient(ellipse at 50%% 80%%,rgba(168,85,247,.06),transparent 60%%);animation:pulse 8s ease-in-out infinite alternate}
@keyframes pulse{0%%{opacity:.6}100%%{opacity:1}}

/* floating orbs */
.orb{position:fixed;border-radius:50%%;filter:blur(80px);opacity:.15;animation:float 20s ease-in-out infinite}
.orb-1{width:400px;height:400px;background:%s;top:-100px;left:-100px;animation-delay:0s}
.orb-2{width:300px;height:300px;background:#3b82f6;bottom:-80px;right:-80px;animation-delay:-7s}
.orb-3{width:200px;height:200px;background:#8b5cf6;top:50%%;left:50%%;animation-delay:-14s}
@keyframes float{0%%,100%%{transform:translate(0,0) scale(1)}25%%{transform:translate(30px,-40px) scale(1.1)}50%%{transform:translate(-20px,30px) scale(.95)}75%%{transform:translate(40px,20px) scale(1.05)}}

/* glassmorphism card */
.card{position:relative;z-index:1;max-width:480px;width:90%%;text-align:center;padding:2.5rem 2rem;background:rgba(255,255,255,.04);backdrop-filter:blur(20px);-webkit-backdrop-filter:blur(20px);border-radius:1.25rem;border:1px solid rgba(255,255,255,.08);box-shadow:0 8px 32px rgba(0,0,0,.3),inset 0 1px 0 rgba(255,255,255,.05)}
.card::before{content:'';position:absolute;inset:0;border-radius:1.25rem;background:linear-gradient(135deg,rgba(255,255,255,.06) 0%%,transparent 50%%);pointer-events:none}

.icon{width:56px;height:56px;margin:0 auto 1.25rem;border-radius:50%%;display:flex;align-items:center;justify-content:center;box-shadow:0 0 30px %s40}
.icon svg{width:28px;height:28px;color:#fff}

h1{font-size:1.2rem;font-weight:700;color:#f1f5f9;letter-spacing:-.02em;margin-bottom:.4rem}

.domain{font-size:.8rem;color:%s;font-family:'SF Mono',Monaco,Consolas,monospace;background:rgba(0,0,0,.3);padding:.5rem 1rem;border-radius:.6rem;margin:1rem 0;word-break:break-all;border:1px solid %s20}

p{font-size:.78rem;color:#94a3b8;line-height:1.7}
.msg{margin-top:.6rem;color:#64748b}

.contact{margin-top:1rem;padding:.6rem 1rem;background:rgba(59,130,246,.08);border:1px solid rgba(59,130,246,.15);border-radius:.5rem;font-size:.75rem;color:#93c5fd}

.footer{margin-top:1.5rem;font-size:.65rem;color:#334155;letter-spacing:.03em}
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
<p>%s</p>
<p class="msg">%s</p>
%s
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
