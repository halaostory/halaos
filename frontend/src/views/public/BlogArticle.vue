<template>
  <div class="blog-article-page">
    <article v-if="article" class="container">
      <div class="article-header">
        <router-link to="/blog" class="back-link">Back to Blog</router-link>
        <div class="article-category-tag">{{ article.categoryLabel }}</div>
        <h1>{{ article.title }}</h1>
        <div class="article-meta">
          <span>{{ article.date }}</span>
          <span class="meta-dot">&middot;</span>
          <span>{{ article.readTime }} min read</span>
        </div>
      </div>
      <div class="article-body" v-html="renderedContent" />
      <div class="article-cta">
        <h3>Ready to automate your HR & payroll?</h3>
        <p>HalaOS handles everything from payroll computation to BIR/IRAS tax filing — completely free.</p>
        <div class="cta-actions">
          <router-link to="/register" class="btn-primary">Get Started Free</router-link>
          <router-link to="/tools" class="btn-outline">Try Free Calculators</router-link>
        </div>
      </div>
    </article>
    <div v-else class="container not-found">
      <h1>Article not found</h1>
      <p>The article you're looking for doesn't exist.</p>
      <router-link to="/blog" class="btn-primary">Back to Blog</router-link>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useHead } from '@unhead/vue'
import MarkdownIt from 'markdown-it'
import { getArticleBySlug } from './blog-data'

const route = useRoute()
const md = new MarkdownIt({ html: false, linkify: true, typographer: true })

const article = computed(() => getArticleBySlug(route.params.slug as string))

const renderedContent = computed(() => {
  if (!article.value) return ''
  // Strip the leading H1 (title) from content since it's already shown in the header
  const content = article.value.content.replace(/^#\s+.+\n+/, '')
  return md.render(content)
})

useHead(computed(() => {
  if (!article.value) return { title: 'Article Not Found - HalaOS Blog' }
  return {
    title: `${article.value.title} - HalaOS Blog`,
    meta: [
      { name: 'description', content: article.value.excerpt },
      { property: 'og:title', content: article.value.title },
      { property: 'og:description', content: article.value.excerpt },
      { property: 'og:url', content: `https://halaos.com/blog/${article.value.slug}` },
      { property: 'og:type', content: 'article' },
    ],
  }
}))
</script>

<style scoped>
.container {
  max-width: 780px;
  margin: 0 auto;
  padding: 0 24px;
}
.blog-article-page {
  padding: 80px 0 60px;
}
.back-link {
  display: inline-block;
  color: #4f46e5;
  font-size: 14px;
  font-weight: 500;
  text-decoration: none;
  margin-bottom: 24px;
}
.back-link:hover { text-decoration: underline; }
.article-category-tag {
  display: inline-block;
  background: #eef2ff;
  color: #4f46e5;
  font-size: 12px;
  font-weight: 600;
  padding: 4px 12px;
  border-radius: 12px;
  margin-bottom: 16px;
}
.article-header h1 {
  font-size: 40px;
  font-weight: 800;
  color: #0f172a;
  line-height: 1.2;
  margin: 0 0 16px;
  letter-spacing: -0.5px;
}
.article-meta {
  font-size: 14px;
  color: #94a3b8;
  margin-bottom: 40px;
}
.meta-dot { margin: 0 8px; }

/* Article Body (rendered markdown) */
.article-body {
  color: #334155;
  font-size: 16px;
  line-height: 1.8;
}
.article-body :deep(h2) {
  font-size: 26px;
  font-weight: 700;
  color: #0f172a;
  margin: 40px 0 16px;
}
.article-body :deep(h3) {
  font-size: 20px;
  font-weight: 700;
  color: #0f172a;
  margin: 32px 0 12px;
}
.article-body :deep(p) {
  margin: 0 0 16px;
}
.article-body :deep(ul),
.article-body :deep(ol) {
  padding-left: 24px;
  margin: 0 0 16px;
}
.article-body :deep(li) {
  margin-bottom: 6px;
}
.article-body :deep(table) {
  width: 100%;
  border-collapse: collapse;
  margin: 16px 0 24px;
  font-size: 14px;
}
.article-body :deep(th),
.article-body :deep(td) {
  padding: 10px 14px;
  border: 1px solid #e2e8f0;
  text-align: left;
}
.article-body :deep(th) {
  background: #f8fafc;
  font-weight: 600;
  color: #0f172a;
}
.article-body :deep(strong) {
  color: #0f172a;
}
.article-body :deep(a) {
  color: #4f46e5;
  text-decoration: none;
}
.article-body :deep(a:hover) {
  text-decoration: underline;
}
.article-body :deep(code) {
  background: #f1f5f9;
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 14px;
}
.article-body :deep(blockquote) {
  border-left: 3px solid #4f46e5;
  padding-left: 16px;
  color: #64748b;
  margin: 16px 0;
}

/* CTA */
.article-cta {
  margin-top: 48px;
  padding: 32px;
  background: #f8fafc;
  border-radius: 12px;
  border: 1px solid #e2e8f0;
  text-align: center;
}
.article-cta h3 {
  font-size: 22px;
  font-weight: 700;
  color: #0f172a;
  margin: 0 0 8px;
}
.article-cta p {
  font-size: 15px;
  color: #64748b;
  margin-bottom: 20px;
}
.cta-actions {
  display: flex;
  gap: 12px;
  justify-content: center;
}
.btn-primary {
  display: inline-block;
  background: #4f46e5;
  color: #fff;
  font-weight: 600;
  font-size: 15px;
  padding: 12px 28px;
  border-radius: 8px;
  text-decoration: none;
  transition: background 0.2s;
}
.btn-primary:hover { background: #4338ca; }
.btn-outline {
  display: inline-block;
  border: 1.5px solid #e2e8f0;
  color: #334155;
  font-weight: 600;
  font-size: 15px;
  padding: 11px 28px;
  border-radius: 8px;
  text-decoration: none;
  transition: all 0.2s;
}
.btn-outline:hover { border-color: #4f46e5; color: #4f46e5; }

/* Not found */
.not-found {
  text-align: center;
  padding: 120px 0;
}
.not-found h1 {
  font-size: 32px;
  font-weight: 700;
  color: #0f172a;
  margin: 0 0 12px;
}
.not-found p {
  color: #64748b;
  margin-bottom: 24px;
}

@media (max-width: 768px) {
  .article-header h1 { font-size: 28px; }
  .cta-actions { flex-direction: column; align-items: center; }
}
</style>
