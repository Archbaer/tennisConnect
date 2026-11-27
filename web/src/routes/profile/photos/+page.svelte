<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON, type Photo } from '#lib/api';
  import PhotoManager from '#lib/PhotoManager.svelte';

  let photos: Photo[] = [];
  let loading = true;

  $: approved = photos.filter((photo) => photo.status === 'approved').length;
  $: pending = photos.filter((photo) => photo.status === 'pending').length;
  $: rejected = photos.filter((photo) => photo.status === 'rejected').length;

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
  <header class="page-head">
    <div>
      <p class="kicker">Your card</p>
      <h1>Photos</h1>
    </div>
    <a href="/profile">Done</a>
  </header>

  {#if loading}
    <p role="status">Loading photos…</p>
  {:else}
    <section class="kpi-strip three" aria-label="Photo status">
      <article class="kpi">
        <h2 class="kpi-label">Approved</h2>
        <p class="kpi-value">{approved}</p>
        <p class="kpi-delta">Visible in Discover</p>
      </article>
      <article class="kpi">
        <h2 class="kpi-label">Pending</h2>
        <p class="kpi-value">{pending}</p>
        <p class="kpi-delta">{pending ? 'Still being checked' : 'Nothing waiting'}</p>
      </article>
      <article class="kpi">
        <h2 class="kpi-label">Slots used</h2>
        <p class="kpi-value">{photos.length}<span class="kpi-unit">/3</span></p>
        <p class="kpi-delta">{rejected ? `${rejected} not accepted` : 'Up to 3 photos'}</p>
      </article>
    </section>
    <PhotoManager {photos} onChange={refresh} />
  {/if}
</section>
