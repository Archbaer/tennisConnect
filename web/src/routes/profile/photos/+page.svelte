<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON, type Photo } from '#lib/api';
  import PhotoManager from '#lib/PhotoManager.svelte';

  let photos: Photo[] = [];
  let loading = true;

  onMount(async () => {
    try {
      photos = (await apiJSON<{ photos: Photo[] }>('/api/v1/photos')).photos;
    } catch {
      photos = [];
    }
    loading = false;
  });

  async function refresh() {
    try {
      photos = (await apiJSON<{ photos: Photo[] }>('/api/v1/photos')).photos;
    } catch {
      photos = [];
    }
  }
</script>

<svelte:head>
  <title>Photos · Tennis Connect</title>
</svelte:head>

<section class="page narrow stack">
  <div class="spread">
    <div>
      <p class="score">Your card</p>
      <h1>Photos</h1>
    </div>
    <a href="/profile">Done</a>
  </div>
  {#if loading}
    <p role="status">Loading photos…</p>
  {:else}
    <PhotoManager {photos} onChange={refresh} />
  {/if}
</section>
