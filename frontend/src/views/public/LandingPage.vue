<script setup lang="ts">
import { defineAsyncComponent } from 'vue'
import { edition } from '../../edition'

// Each product has its own home page; the edition is loaded before routing.
const HomePage = defineAsyncComponent(() => import('./HomePage.vue'))
const PeopleHomePage = defineAsyncComponent(() => import('./PeopleHomePage.vue'))
</script>

<template>
  <PeopleHomePage v-if="edition.config.edition === 'people'" />
  <HomePage v-else />
</template>
