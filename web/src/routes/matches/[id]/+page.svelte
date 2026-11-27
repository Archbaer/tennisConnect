<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { apiJSON, relativeDate, type Match } from '#lib/api';
  import SafetyDialog from '#lib/SafetyDialog.svelte';

  let match: Match | null = null;
  let matchID = '';
  let handle = '';
  let error = '';
  let copied = '';
  let loading = true;
  let safety: { openReport: () => Promise<void>; openBlock: () => Promise<void> };

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
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'Instagram is unavailable.';
    }
  }

  async function copyHandle() {
    await navigator.clipboard.writeText(`@${handle}`);
    copied = `Copied @${handle}`;
    setTimeout(() => (copied = ''), 2500);
  }

  async function openSafety(mode: 'report' | 'block') {
    if (mode === 'block') {
      await safety.openBlock();
    } else {
      await safety.openReport();
    }
  }
</script>

<svelte:head>
  <title>Match · Tennis Connect</title>
</svelte:head>

<section class="page narrow stack">
  {#if loading}
    <div class="card p-3" aria-hidden="true">
      <div class="placeholder-glow">
        <p class="placeholder col-6"></p>
        <p class="placeholder col-10"></p>
      </div>
    </div>
    <p class="visually-hidden" role="status">Loading match…</p>
  {:else if error && !match}
    <div class="alert alert-danger stack" role="alert">
      <p class="mb-0">{error}</p>
      <a class="btn btn-outline-danger align-self-start" href="/matches">Back to matches</a>
    </div>
  {:else if match}
    <a class="back" href="/matches"><i class="bi bi-arrow-left" aria-hidden="true"></i> Matches</a>

    <header class="page-head">
      <div>
        <p class="kicker">Mutual match</p>
        <h1 class="mb-1">{match.display_name}</h1>
        <p class="muted mb-0">Matched {relativeDate(match.matched_at)}</p>
      </div>
      <i class="bi bi-people match-icon" aria-hidden="true"></i>
    </header>

    <section class="kpi-strip" aria-label="Match stats">
      <article class="kpi">
        <h2 class="kpi-label">Visibility</h2>
        <p class="kpi-value small">{match.revealed ? 'Revealed' : 'New'}</p>
        <p class="kpi-delta">{match.revealed ? 'You have both opted in' : 'Open to reveal'}</p>
      </article>
      <article class="kpi">
        <h2 class="kpi-label">Instagram</h2>
        <p class="kpi-value small">{match.has_instagram ? 'Shared' : 'None'}</p>
        <p class="kpi-delta">{match.has_instagram ? 'Handle available on request' : 'Not added'}</p>
      </article>
      <article class="kpi">
        <h2 class="kpi-label">Photos</h2>
        <p class="kpi-value">{match.photo_urls.length}</p>
        <p class="kpi-delta">Approved photos</p>
      </article>
      <article class="kpi">
        <h2 class="kpi-label">Matched</h2>
        <p class="kpi-value small">{relativeDate(match.matched_at)}</p>
        <p class="kpi-delta">Both said yes</p>
      </article>
    </section>

    {#if match.photo_urls.length}
      <div class="photo-grid">
        {#each match.photo_urls as url, index}
          <img class="photo" src={url} alt={`${match.display_name}, photo ${index + 1}`} />
        {/each}
      </div>
    {/if}

    {#if error}
      <p class="alert alert-danger" role="alert">{error}</p>
    {/if}

    <div class="sheet-card stack">
      <div class="d-flex align-items-center gap-2">
        <i class="bi bi-instagram" aria-hidden="true"></i>
        <h2 class="mb-0">Instagram</h2>
      </div>
      <p class="muted mb-0">
        Shared because you both matched. It is never included in the match list.
      </p>
      {#if handle}
        <p class="instagram mb-0">@{handle}</p>
        <div class="d-flex flex-wrap gap-2">
          <button class="btn btn-primary" onclick={copyHandle} aria-label={`Copy ${match.display_name}'s Instagram handle`}>
            <i class="bi bi-clipboard" aria-hidden="true"></i> Copy
          </button>
          <a class="btn btn-outline-secondary" href={`https://instagram.com/${handle}`} target="_blank" rel="noopener noreferrer" referrerpolicy="no-referrer">
            Open Instagram
          </a>
        </div>
        <p class="visually-hidden" aria-live="polite">{copied}</p>
      {:else if match.has_instagram}
        <button class="btn btn-primary align-self-start" onclick={showInstagram}>Show Instagram</button>
      {:else}
        <p class="notice mb-0">
          {match.display_name} hasn't added Instagram. Tennis Connect doesn't have chat yet, so meet on court at the times above.
        </p>
      {/if}
    </div>

    <div class="sheet-card stack">
      <h2>Safety</h2>
      <p class="muted">Blocking closes the match for both of you and stops new photo and handle access.</p>
      <div class="d-flex flex-wrap gap-2">
        <button class="btn btn-outline-secondary" onclick={() => openSafety('report')}>
          <i class="bi bi-flag" aria-hidden="true"></i> Report {match.display_name}
        </button>
        <button class="btn btn-outline-danger" onclick={() => openSafety('block')}>
          <i class="bi bi-slash-circle" aria-hidden="true"></i> Block {match.display_name}
        </button>
      </div>
    </div>

    <SafetyDialog
      bind:this={safety}
      targetId={match.player_id}
      targetName={match.display_name}
      onBlocked={() => goto('/matches')}
    />
  {/if}
</section>

<style>
  .back {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    font-weight: 700;
    text-decoration: none;
  }

  .match-icon {
    color: var(--success);
  }

  .instagram {
    font-size: 1.5rem;
    font-weight: 800;
    letter-spacing: -0.02em;
  }
</style>
