<script setup lang="ts">
const year = new Date().getFullYear()

const links = {
  product: [
    { label: 'Features', to: '/features' },
    { label: 'Pricing', to: '/pricing' },
    { label: 'Free Tools', to: '/tools' },
    { label: 'Blog', to: '/blog' },
  ],
  resources: [
    { label: 'Tax Calculator', to: '/tools/tax-calculator' },
    { label: 'SSS Calculator', to: '/tools/sss-calculator' },
    { label: '13th Month Pay', to: '/tools/13th-month-calculator' },
    { label: 'BIR 2550M Guide', to: '/blog/bir-2550m-guide-2026' },
  ],
  company: [
    { label: 'About', to: '/about' },
    { label: 'Contact', to: '/contact' },
    { label: 'GitHub', to: 'https://github.com/halaostory/halaos' },
    { label: 'Privacy Policy', to: '/privacy' },
    { label: 'Terms of Service', to: '/terms' },
  ],
}
</script>

<template>
  <footer class="pub-footer">
    <div class="footer-inner">
      <div class="footer-grid">
        <div class="footer-brand">
          <router-link to="/" class="footer-logo">
            <span class="logo-icon">H</span>
            <span class="logo-text">HalaOS</span>
          </router-link>
          <p class="footer-tagline">
            Unified HR, Payroll &amp; Tax Compliance platform for growing businesses across Asia and the US. Open source on GitHub.
          </p>
          <div class="footer-social">
            <a href="https://twitter.com/halaos" target="_blank" rel="noopener" aria-label="Twitter">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor"><path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z"/></svg>
            </a>
            <a href="https://linkedin.com/company/halaos" target="_blank" rel="noopener" aria-label="LinkedIn">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor"><path d="M20.447 20.452h-3.554v-5.569c0-1.328-.027-3.037-1.852-3.037-1.853 0-2.136 1.445-2.136 2.939v5.667H9.351V9h3.414v1.561h.046c.477-.9 1.637-1.85 3.37-1.85 3.601 0 4.267 2.37 4.267 5.455v6.286zM5.337 7.433c-1.144 0-2.063-.926-2.063-2.065 0-1.138.92-2.063 2.063-2.063 1.14 0 2.064.925 2.064 2.063 0 1.139-.925 2.065-2.064 2.065zm1.782 13.019H3.555V9h3.564v11.452zM22.225 0H1.771C.792 0 0 .774 0 1.729v20.542C0 23.227.792 24 1.771 24h20.451C23.2 24 24 23.227 24 22.271V1.729C24 .774 23.2 0 22.222 0h.003z"/></svg>
            </a>
            <a href="https://github.com/halaostory/halaos" target="_blank" rel="noopener" aria-label="GitHub">
              <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor"><path d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12"/></svg>
            </a>
          </div>
        </div>

        <div class="footer-links-group">
          <h4>Product</h4>
          <ul>
            <li v-for="link in links.product" :key="link.to">
              <router-link :to="link.to">{{ link.label }}</router-link>
            </li>
          </ul>
        </div>

        <div class="footer-links-group">
          <h4>Free Resources</h4>
          <ul>
            <li v-for="link in links.resources" :key="link.to">
              <router-link :to="link.to">{{ link.label }}</router-link>
            </li>
          </ul>
        </div>

        <div class="footer-links-group">
          <h4>Company</h4>
          <ul>
            <li v-for="link in links.company" :key="link.to">
              <a v-if="link.to.startsWith('http')" :href="link.to" target="_blank" rel="noopener">{{ link.label }}</a>
              <router-link v-else :to="link.to">{{ link.label }}</router-link>
            </li>
          </ul>
        </div>
      </div>

      <div class="footer-bottom">
        <p>&copy; {{ year }} HalaOS. All rights reserved.</p>
        <p class="footer-regions">Philippines &middot; Singapore &middot; Sri Lanka &middot; Indonesia &middot; United States</p>
      </div>
    </div>
  </footer>
</template>

<style scoped>
.pub-footer {
  background: #0f172a;
  color: #94a3b8;
  padding: 64px 0 32px;
}
.footer-inner {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 24px;
}
.footer-grid {
  display: grid;
  grid-template-columns: 2fr 1fr 1fr 1fr;
  gap: 40px;
}
.footer-logo {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  text-decoration: none;
  font-weight: 700;
  font-size: 20px;
  color: #fff;
}
.footer-logo .logo-icon {
  width: 32px;
  height: 32px;
  background: linear-gradient(135deg, #4f46e5, #7c3aed);
  color: #fff;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  font-weight: 800;
}
.footer-tagline {
  margin: 16px 0 24px;
  line-height: 1.6;
  font-size: 14px;
  max-width: 300px;
}
.footer-social {
  display: flex;
  gap: 16px;
}
.footer-social a {
  color: #64748b;
  transition: color 0.2s;
}
.footer-social a:hover {
  color: #e2e8f0;
}
.footer-links-group h4 {
  color: #e2e8f0;
  font-size: 14px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin-bottom: 16px;
}
.footer-links-group ul {
  list-style: none;
  padding: 0;
  margin: 0;
}
.footer-links-group li {
  margin-bottom: 10px;
}
.footer-links-group a {
  color: #94a3b8;
  text-decoration: none;
  font-size: 14px;
  transition: color 0.2s;
}
.footer-links-group a:hover {
  color: #e2e8f0;
}
.footer-bottom {
  margin-top: 48px;
  padding-top: 24px;
  border-top: 1px solid #1e293b;
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 13px;
}
.footer-regions {
  color: #64748b;
}

@media (max-width: 768px) {
  .footer-grid {
    grid-template-columns: 1fr 1fr;
    gap: 32px;
  }
  .footer-brand {
    grid-column: 1 / -1;
  }
  .footer-bottom {
    flex-direction: column;
    gap: 8px;
    text-align: center;
  }
}

@media (max-width: 480px) {
  .footer-grid {
    grid-template-columns: 1fr;
  }
}
</style>
