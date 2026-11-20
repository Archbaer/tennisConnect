<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { apiJSON, formatDate, type Match } from '#lib/api';

  let match: Match | null = null;
  let matchID = '';
  let handle = '';
  let error = '';
  let loading = true;
  let confirmingBlock = false;

  onMount(async () => {
    matchID = window.location.pathname.split('/').filter(Boolean).pop() ?? '';
    try {
      match = await apiJSON<Match>(`/api/v1/matches/${matchID}`);
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'That match is unavailable.';
    } finally {
      loading = false;
    }
  });

  async function showInstagram() {
    try {
      const result = await apiJSON<{ instagram_handle: string }>(
        `/api/v1/matches/${matchID}/instagram`
      );
      handle = result.instagram_handle;
      if (!handle) {
        error = 'They have not added an Instagram handle.';
      }
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'Instagram is unavailable.';
    }
  }

  async function block() {
    if (!match) return;
    try {
      await apiJSON('/api/v1/blocks', {
        method: 'POST',
        body: JSON.stringify({ target_id: match.player_id })
      });
      await goto('/matches');
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'We could not block that player.';
    }
  }
</script>

<svelte:head>
  <title>Match · Tennis Connect</title>
</svelte:head>

<section class="page narrow stack">
  {#if loading}
    <p role="status">Loading match…</p>
  {:else if error && !match}
    <div class="alert-box stack" role="alert">
      <p>{error}</p>
      <a class="button" href="/matches">Back to matches</a>
    </div>
  {:else if match}
    <p class="score">Mutual match</p>
    <h1>{match.display_name}</h1>
    <p class="muted">Matched {formatDate(match.matched_at)}</p>

    {#if match.photo_urls.length}
      <div class="photo-grid">
        {#each match.photo_urls as url, index}
          <img class="photo" src={url} alt={`${match.display_name}, photo ${index + 1}`} />
        {/each}
      </div>
    {/if}

    {#if error}
      <p class="alert-box" role="alert">{error}</p>
    {/if}

    <div class="surface stack">
      <h2>Instagram</h2>
      <p class="muted">
        The handle is released only through this action, only while the match exists. It is never
        included in the match list.
      </p>
      {#if handle}
        <p class="instagram" aria-live="polite">@{handle}</p>
      {:else if match.has_instagram}
        <button onclick={showInstagram}>Show Instagram</button>
      {:else}
        <p class="notice">They have not added an Instagram handle.</p>
      {/if}
    </div>

    <div class="surface stack">
      <h2>Safety</h2>
      <p class="muted">Blocking closes the match for both of you and stops new photo and handle access.</p>
      {#if confirmingBlock}
        <div class="row">
          <button class="danger" onclick={block}>Confirm block</button>
          <button class="ghost" onclick={() => (confirmingBlock = false)}>Cancel</button>
        </div>
      {:else}
        <div class="row">
          <a class="button secondary" href={`/report?target=${match.player_id}`}>Report</a>
          <button class="secondary" onclick={() => (confirmingBlock = true)}>Block</button>
        </div>
      {/if}
    </div>
  {/if}
</section>

<style>
  .instagram {
    font-size: 1.35rem;
    font-weight: 700;
  }
</style>
