<template>
  <div class="blog-page">
    <section class="blog-hero">
      <div class="container">
        <h1>HalaOS Blog</h1>
        <p>Guides, tips, and updates on HR, payroll, and tax compliance in Southeast Asia.</p>
      </div>
    </section>

    <!-- Category Filter -->
    <section class="blog-filter">
      <div class="container">
        <div class="filter-row">
          <button
            v-for="cat in categories"
            :key="cat.value"
            class="filter-btn"
            :class="{ active: activeCategory === cat.value }"
            @click="activeCategory = cat.value"
          >{{ cat.label }}</button>
        </div>
      </div>
    </section>

    <!-- Articles Grid -->
    <section class="blog-list">
      <div class="container">
        <div class="articles-grid">
          <router-link
            v-for="article in filteredArticles"
            :key="article.slug"
            :to="`/blog/${article.slug}`"
            class="article-card"
          >
            <div class="article-category">{{ article.categoryLabel }}</div>
            <h2>{{ article.title }}</h2>
            <p>{{ article.excerpt }}</p>
            <div class="article-meta">
              <span>{{ article.date }}</span>
              <span class="meta-dot">&middot;</span>
              <span>{{ article.readTime }} min read</span>
            </div>
          </router-link>
        </div>
      </div>
    </section>

    <!-- CTA -->
    <section class="blog-cta">
      <div class="container">
        <div class="cta-card">
          <h2>Automate your HR & payroll today</h2>
          <p>Stop computing taxes manually. Let HalaOS handle BIR forms, SSS, PhilHealth, and Pag-IBIG automatically.</p>
          <router-link to="/register" class="btn-primary">Get Started Free</router-link>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useHead } from '@unhead/vue'
import { blogArticles } from './blog-data'

useHead({
  title: 'Blog - HalaOS | HR, Payroll & Tax Guides',
  meta: [
    { name: 'description', content: 'Expert guides on BIR compliance, SSS contributions, PhilHealth, payroll processing, CPF calculations, and HR management for Philippine and Singapore businesses.' },
    { property: 'og:title', content: 'Blog - HalaOS | HR, Payroll & Tax Guides' },
    { property: 'og:url', content: 'https://halaos.com/blog' },
  ],
})

const activeCategory = ref('all')

const categories = [
  { label: 'All', value: 'all' },
  { label: 'BIR Compliance', value: 'bir' },
  { label: 'PH Payroll', value: 'ph-payroll' },
  { label: 'SG Compliance', value: 'sg' },
  { label: 'HR Management', value: 'hr' },
  { label: 'Guides', value: 'guides' },
]

const filteredArticles = computed(() => {
  if (activeCategory.value === 'all') return blogArticles
  return blogArticles.filter(a => a.category === activeCategory.value)
})
</script>

<style scoped>
.container {
  max-width: 1200px;
  margin: 0 auto;
  padding: 0 24px;
}
.blog-hero {
  padding: 100px 0 40px;
  text-align: center;
  background: linear-gradient(180deg, #f8fafc 0%, #fff 100%);
}
.blog-hero h1 {
  font-size: 48px;
  font-weight: 800;
  color: #0f172a;
  margin: 0 0 12px;
  letter-spacing: -1px;
}
.blog-hero p {
  font-size: 18px;
  color: #475569;
  max-width: 560px;
  margin: 0 auto;
}

/* Filter */
.blog-filter {
  padding: 24px 0;
  border-bottom: 1px solid #f1f5f9;
}
.filter-row {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: center;
}
.filter-btn {
  background: none;
  border: 1px solid #e2e8f0;
  border-radius: 20px;
  padding: 8px 18px;
  font-size: 14px;
  font-weight: 500;
  color: #475569;
  cursor: pointer;
  transition: all 0.2s;
}
.filter-btn:hover { border-color: #4f46e5; color: #4f46e5; }
.filter-btn.active {
  background: #4f46e5;
  color: #fff;
  border-color: #4f46e5;
}

/* Articles */
.blog-list {
  padding: 48px 0;
}
.articles-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 24px;
}
.article-card {
  background: #fff;
  border: 1px solid #e2e8f0;
  border-radius: 12px;
  padding: 28px;
  text-decoration: none;
  transition: all 0.2s;
  display: block;
}
.article-card:hover {
  border-color: #4f46e5;
  box-shadow: 0 4px 16px rgba(79,70,229,0.08);
}
.article-category {
  display: inline-block;
  background: #eef2ff;
  color: #4f46e5;
  font-size: 12px;
  font-weight: 600;
  padding: 4px 10px;
  border-radius: 12px;
  margin-bottom: 12px;
}
.article-card h2 {
  font-size: 18px;
  font-weight: 700;
  color: #0f172a;
  margin: 0 0 8px;
  line-height: 1.4;
}
.article-card p {
  font-size: 14px;
  color: #64748b;
  line-height: 1.6;
  margin: 0 0 16px;
}
.article-meta {
  font-size: 13px;
  color: #94a3b8;
}
.meta-dot { margin: 0 6px; }

/* CTA */
.blog-cta {
  padding: 0 0 80px;
}
.cta-card {
  background: linear-gradient(135deg, #4f46e5, #7c3aed);
  border-radius: 16px;
  padding: 48px;
  text-align: center;
  color: #fff;
}
.cta-card h2 {
  font-size: 28px;
  font-weight: 800;
  margin: 0 0 12px;
}
.cta-card p {
  font-size: 16px;
  opacity: 0.9;
  margin-bottom: 24px;
  max-width: 500px;
  margin-left: auto;
  margin-right: auto;
}
.btn-primary {
  display: inline-block;
  background: #fff;
  color: #4f46e5;
  font-weight: 600;
  font-size: 15px;
  padding: 12px 28px;
  border-radius: 8px;
  text-decoration: none;
  transition: background 0.2s;
}
.btn-primary:hover { background: #f1f5f9; }

@media (max-width: 768px) {
  .blog-hero h1 { font-size: 32px; }
  .articles-grid { grid-template-columns: 1fr; }
}
</style>
