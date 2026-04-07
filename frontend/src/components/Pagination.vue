<script setup>
import { computed } from 'vue';

const props = defineProps({
  currentPage: {
    type: Number,
    required: true
  },
  totalCount: {
    type: Number,
    required: true
  },
  pageSize: {
    type: Number,
    required: true
  },
  onGotoPage: {
    type: Function,
    required: true
  },
  onNextPage: {
    type: Function,
    required: true
  },
  onPrevPage: {
    type: Function,
    required: true
  },
  hasNextPage: {
    type: Boolean,
    required: true
  },
  hasPrevPage: {
    type: Boolean,
    required: true
  }
});

const totalPages = computed(() => Math.ceil(props.totalCount / props.pageSize));

const pages = computed(() => {
  const current = props.currentPage;
  const last = totalPages.value;
  const delta = 2; // Number of pages to show before and after current page
  
  const range = [];
  const rangeWithDots = [];

  // If total pages is small, just show all pages
  if (last <= 7) {
    for (let i = 1; i <= last; i++) {
      rangeWithDots.push({ type: 'page', value: i });
    }
    return rangeWithDots;
  }

  // Calculate the range around the current page
  let start = Math.max(2, current - delta);
  let end = Math.min(last - 1, current + delta);

  // Adjust range to always show a consistent number of pages if possible
  if (current <= delta + 2) {
    end = 2 + delta * 2;
  } else if (current >= last - delta - 1) {
    start = last - delta * 2 - 1;
  }

  // Add dots before the range if needed
  if (start > 2) {
    rangeWithDots.push({ type: 'page', value: 1 });
    rangeWithDots.push({ type: 'dots', value: 'dots-start' });
  } else {
    for (let i = 1; i < start; i++) {
      rangeWithDots.push({ type: 'page', value: i });
    }
  }

  // Add the central range
  for (let i = start; i <= end; i++) {
    rangeWithDots.push({ type: 'page', value: i });
  }

  // Add dots after the range if needed
  if (end < last - 1) {
    rangeWithDots.push({ type: 'dots', value: 'dots-end' });
    rangeWithDots.push({ type: 'page', value: last });
  } else {
    for (let i = end + 1; i <= last; i++) {
      rangeWithDots.push({ type: 'page', value: i });
    }
  }

  return rangeWithDots;
});
</script>

<template>
  <ul class="pagination pagination-sm mb-0" v-if="totalPages > 1">
    <li :class="{ disabled: !hasPrevPage }" class="page-item">
      <a class="page-link" href="#" @click.prevent="hasPrevPage && onPrevPage()">&laquo;</a>
    </li>

    <li v-for="item in pages" :key="item.type === 'page' ? item.value : item.value" :class="{ active: currentPage === item.value, disabled: item.type === 'dots' }" class="page-item">
      <a v-if="item.type === 'page'" class="page-link" href="#" @click.prevent="onGotoPage(item.value)">{{ item.value }}</a>
      <span v-else class="page-link">...</span>
    </li>

    <li :class="{ disabled: !hasNextPage }" class="page-item">
      <a class="page-link" href="#" @click.prevent="hasNextPage && onNextPage()">&raquo;</a>
    </li>
  </ul>
</template>

<style scoped>
.page-link {
  cursor: pointer;
}
.page-item.disabled .page-link {
  cursor: default;
}
</style>
