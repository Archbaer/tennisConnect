<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { goto } from '$app/navigation';
  import { ApiError, apiJSON, distanceLabel, type DiscoveryCard } from '#lib/api';
  import { noteMatchCreated } from '#lib/matchState';
  import { online } from '#lib/offline';
  import { loadRadius, saveRadius } from '#lib/preferences';
  import SafetyDialog from '#lib/SafetyDialog.svelte';

  let cards: DiscoveryCard[] = [];
  let cursor: string | null = null;
  let radius = 10;
  let loading = true;
  let error = '';
  let blockedReason = '';
  let confirmPass: string | null = null;
  let acting: string | null = null;
  let safetyTarget: DiscoveryCard | null = null;
  let safety: SafetyDialog;

  $: card = cards[0];

  onMount(() => {
    radius = loadRadius();
    load();
  });

  async function load(reset = true) {
    loading = true;
    error = '';
    blockedReason = '';
    try {
      const query = new URLSearchParams({ radius: String(radius), limit: '20' });
      if (!reset && cursor) query.set('cursor', cursor);
      const page = await apiJSON<{ cards: DiscoveryCard[]; next_cursor: string | null }>(
        `/api/v1/discovery?${query}`
      );
      cards = reset ? page.cards : [...cards, ...page.cards];
      cursor = page.next_cursor;
    } catch (problem) {
      if (problem instanceof ApiError && problem.status === 409) {
        blockedReason = problem.message;
      } else {
        error = problem instanceof Error ? problem.message : 'Discover is unavailable.';
      }
    } finally {
      loading = false;
    }
  }

  async function react(targetID: string, action: 'like' | 'pass') {
    acting = targetID;
    error = '';
    try {
      const result = await apiJSON<{ matched: boolean; match_id?: string }>('/api/v1/reactions', {
        method: 'POST',
        body: JSON.stringify({ target_id: targetID, action })
      });
      advance(targetID);
      confirmPass = null;
      if (result.matched && result.match_id) {
        noteMatchCreated();
        await goto(`/matches/${result.match_id}/new`);
      }
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'That reaction did not save.';
    } finally {
      acting = null;
    }
  }

  function advance(targetID: string) {
    cards = cards.filter((item) => item.player.player_id !== targetID);
    if (!cards.length && cursor) load(false);
  }

  function initials(name: string): string {
    return name
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((part) => part[0]?.toUpperCase() ?? '')
      .join('');
  }

  async function openSafety(cardItem: DiscoveryCard, mode: 'report' | 'block') {
    safetyTarget = cardItem;
    await tick();
    if (mode === 'block') {
      await safety.openBlock();
    } else {
      await safety.openReport();
    }
  }
</script>

<svelte:head>
  <title>Discover · Tennis Connect</title>
</svelte:head>

<section class="page discover-page stack">
  <div class="discover-head">
    <div>
      <p class="score mb-1"><span class="tennis-ball me-2" aria-hidden="true"></span>Discover</p>
      <h1 class="mb-0">Players near you</h1>
    </div>
    <label class="radius">
      <span class="score">Radius</span>
      <select class="form-select" bind:value={radius} onchange={() => { saveRadius(radius); load(true); }}>
        <option value={5}>5 miles</option>
        <option value={10}>10 miles</option>
        <option value={20}>20 miles</option>
        <option value={30}>30 miles</option>
      </select>
    </label>
  </div>

  {#if blockedReason}
    <div class="alert alert-warning stack" role="alert">
      <strong>Discover is not open yet.</strong>
      <p class="mb-0">{blockedReason}</p>
      <a class="btn btn-primary align-self-start" href="/onboarding">Finish setup</a>
    </div>
  {:else if loading && !cards.length}
    <div class="player-card skeleton-card" aria-hidden="true">
      <div class="skeleton-photo"></div>
      <div class="skeleton-body">
        <p class="placeholder col-7"></p>
        <p class="placeholder col-4"></p>
        <p class="placeholder col-10"></p>
      </div>
    </div>
    <p class="visually-hidden" role="status">Loading players…</p>
  {:else if error}
    <div class="alert alert-danger stack" role="alert">
      <p class="mb-0">{error}</p>
      <button class="btn btn-outline-danger align-self-start" onclick={() => load(true)}>Try again</button>
    </div>
  {:else if !card}
    <div class="surface stack text-center">
      <div class="empty-ball mx-auto" aria-hidden="true"></div>
      <h2>No one in range right now</h2>
      <p class="muted">Try a wider radius, add play times, or check your area.</p>
      <div class="d-flex justify-content-center gap-2">
        <button class="btn btn-primary" onclick={() => { radius = 30; load(true); }}>Widen radius</button>
        <a class="btn btn-outline-secondary" href="/onboarding">Change area</a>
      </div>
    </div>
  {:else}
    <article class="player-card" aria-labelledby={`name-${card.player.player_id}`}>
      <div class="photo-wrap">
        <div class="photo-fallback" aria-hidden="true">
          <span>{initials(card.player.display_name)}</span>
        </div>
        {#if card.player.photo_urls.length}
          <img
            src={card.player.photo_urls[0]}
            alt={`${card.player.display_name}'s photo`}
            onerror={(event) => ((event.currentTarget as HTMLImageElement).style.display = 'none')}
          />
        {/if}
        <span class="chip photo-chip">{distanceLabel(card.player.distance_miles)}</span>
        <button
          class="icon-button card-safety"
          type="button"
          onclick={() => openSafety(card, 'report')}
          aria-label={`Safety options for ${card.player.display_name}`}
        >
          <i class="bi bi-flag" aria-hidden="true"></i>
        </button>
      </div>

      <div class="card-body">
        <div class="name-row">
          <h2 id={`name-${card.player.player_id}`} class="mb-0">
            {card.player.display_name}, {card.player.age}
          </h2>
          <span class="chip">{card.player.skill_level}</span>
        </div>

        <div class="week-strip" aria-hidden="true">
          {#each ['M', 'T', 'W', 'T', 'F', 'S', 'S'] as day, index}
            <span class:active={card.player.play_summary.toLowerCase().includes(['mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun'][index])}>{day}</span>
          {/each}
        </div>

        <dl class="meta">
          <div>
            <dt class="score">Plays</dt>
            <dd>{card.player.play_summary || 'No shared times listed'}</dd>
          </div>
          <div>
            <dt class="score">Distance</dt>
            <dd>{distanceLabel(card.player.distance_miles)}</dd>
          </div>
        </dl>

        {#if card.player.bio}<p class="bio muted">{card.player.bio}</p>{/if}
        <a class="profile-link" href={`/discover/${card.player.player_id}?radius=${radius}`}>
          View full profile
        </a>

        {#if error}
          <p class="alert alert-danger py-2 mb-0" role="alert">{error}</p>
        {/if}

        {#if confirmPass === card.player.player_id}
          <div class="confirm-pass">
            <p class="small mb-2">You won't see each other again.</p>
            <div class="d-flex gap-2">
              <button class="btn btn-danger flex-grow-1" onclick={() => react(card.player.player_id, 'pass')} disabled={acting === card.player.player_id}>
                Not for me
              </button>
              <button class="btn btn-outline-secondary" onclick={() => (confirmPass = null)}>Cancel</button>
            </div>
          </div>
        {:else}
          <div class="card-actions">
            <button
              class="btn btn-primary"
              onclick={() => react(card.player.player_id, 'like')}
              disabled={acting === card.player.player_id || !$online}
            >
              {acting === card.player.player_id ? 'Sending…' : "I'd play"}
            </button>
            <button
              class="btn btn-outline-secondary"
              onclick={() => (confirmPass = card.player.player_id)}
              disabled={!$online}
            >
              Not for me
            </button>
          </div>
        {/if}
      </div>
    </article>

    {#if cursor}
      <button class="btn btn-outline-secondary w-100" onclick={() => load(false)} disabled={loading}>
        {loading ? 'Loading…' : 'More players'}
      </button>
    {/if}
  {/if}

  <SafetyDialog
    bind:this={safety}
    targetId={safetyTarget?.player.player_id ?? ''}
    targetName={safetyTarget?.player.display_name ?? 'this player'}
    onBlocked={() => {
      if (safetyTarget) advance(safetyTarget.player.player_id);
    }}
    onReported={() => {
      if (safetyTarget) advance(safetyTarget.player.player_id);
    }}
  />
</section>

<style>
  .discover-page {
    width: min(100% - 1.5rem, 30rem);
    padding-block: 1rem 1.5rem;
  }

  .discover-head {
    display: flex;
    align-items: flex-end;
    justify-content: space-between;
    gap: 0.75rem;
  }

  .discover-head h1 {
    font-size: clamp(1.5rem, 4vw, 2rem);
    line-height: 1.05;
  }

  .radius {
    width: 7rem;
    flex: 0 0 auto;
  }

  .radius .form-select {
    min-height: 2.6rem;
    padding-block: 0.4rem;
  }

  .player-card {
    display: flex;
    flex-direction: column;
    overflow: hidden;
    border: 1px solid var(--line);
    border-radius: var(--radius);
    background: var(--surface);
    box-shadow: var(--shadow);
  }

  .photo-wrap {
    position: relative;
    aspect-ratio: 4 / 3;
    max-height: 20rem;
    overflow: hidden;
    background: #dfe9df;
  }

  .photo-wrap img {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  .photo-fallback {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
    background:
      linear-gradient(135deg, rgb(216 255 62 / 18%), transparent 55%),
      linear-gradient(160deg, var(--court-light), var(--court-deep));
    color: var(--ball);
    font-size: clamp(4rem, 18vw, 7rem);
    font-weight: 800;
    letter-spacing: -0.06em;
  }

  .photo-fallback::before {
    content: '';
    position: absolute;
    inset: 8%;
    border: 2px solid rgb(247 249 244 / 32%);
    border-radius: 50%;
    transform: rotate(-12deg);
  }

  .photo-chip {
    position: absolute;
    left: 0.75rem;
    top: 0.75rem;
    background: rgb(255 255 255 / 92%);
    backdrop-filter: blur(6px);
  }

  .card-safety {
    position: absolute;
    right: 0.75rem;
    top: 0.75rem;
  }

  .card-body {
    display: grid;
    gap: 0.55rem;
    padding: 1rem;
  }

  .name-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
  }

  .name-row h2 {
    font-size: clamp(1.45rem, 5.5vw, 1.85rem);
  }

  .week-strip {
    display: grid;
    grid-template-columns: repeat(7, 1fr);
    gap: 0.3rem;
  }

  .week-strip span {
    display: grid;
    place-items: center;
    min-height: 1.75rem;
    border: 1px solid var(--line);
    border-radius: var(--radius-chip);
    color: var(--muted);
    font-size: 0.72rem;
    font-weight: 700;
  }

  .week-strip span.active {
    border-color: var(--court);
    background: var(--court);
    color: white;
  }

  .meta {
    display: grid;
    gap: 0.35rem;
    margin: 0;
    padding-block: 0.45rem;
    border-block: 1px solid var(--line);
  }

  .meta > div {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 1rem;
  }

  .meta dt {
    color: var(--muted);
  }

  .meta dd {
    margin: 0;
    text-align: right;
    font-weight: 700;
  }

  .profile-link {
    justify-self: start;
    font-weight: 700;
    text-decoration: underline;
    text-underline-offset: 0.18em;
  }

  .bio {
    display: -webkit-box;
    overflow: hidden;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
    line-clamp: 2;
    margin: 0;
  }

  .card-actions {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.6rem;
  }

  .card-actions .btn {
    min-height: 2.8rem;
  }

  .confirm-pass {
    padding: 0.75rem;
    border: 1px solid #e6c4bf;
    border-radius: var(--radius-control);
    background: #fbefeb;
  }

  .skeleton-card {
    min-height: 32rem;
  }

  .skeleton-photo {
    aspect-ratio: 4 / 3;
    max-height: 20rem;
    background: #e3ece5;
  }

  .skeleton-body {
    padding: 1.15rem;
  }

  @media (max-width: 360px) {
    .discover-head {
      align-items: stretch;
      flex-direction: column;
    }

    .radius {
      width: 100%;
    }

    .card-actions {
      grid-template-columns: 1fr;
    }
  }
</style>
