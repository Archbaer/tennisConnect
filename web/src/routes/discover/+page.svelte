<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { goto } from '$app/navigation';
  import { ApiError, apiJSON, distanceLabel, type DiscoveryCard } from '#lib/api';
  import LoadingSkeleton from '#lib/LoadingSkeleton.svelte';
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
  <header class="discover-head">
    <div>
      <p class="kicker">Discover</p>
      <h1 class="mb-0">Players near you</h1>
    </div>
    <label class="radius">
      <span class="kicker">Radius</span>
      <select class="form-select" bind:value={radius} onchange={() => { saveRadius(radius); load(true); }}>
        <option value={5}>5 miles</option>
        <option value={10}>10 miles</option>
        <option value={20}>20 miles</option>
        <option value={30}>30 miles</option>
      </select>
    </label>
  </header>

  <section class="kpi-strip three" aria-label="Discovery tracker">
    <article class="kpi">
      <h2 class="kpi-label">Radius</h2>
      <p class="kpi-value">{radius}<span class="kpi-unit">mi</span></p>
      <p class="kpi-delta">Search area</p>
    </article>
    <article class="kpi">
      <h2 class="kpi-label">In deck</h2>
      <p class="kpi-value">{cards.length}</p>
      <p class="kpi-delta">{cursor ? 'More available' : 'Last cards'}</p>
    </article>
    <article class="kpi">
      <h2 class="kpi-label">Waiting</h2>
      <p class="kpi-value">{Math.max(cards.length - 1, 0)}</p>
      <p class="kpi-delta">After this card</p>
    </article>
  </section>

  {#if blockedReason}
    <div class="alert alert-warning stack" role="alert">
      <strong>Discover is not open yet.</strong>
      <p class="mb-0">{blockedReason}</p>
      <a class="btn btn-primary align-self-start" href="/onboarding">Finish setup</a>
    </div>
  {:else if loading && !cards.length}
    <LoadingSkeleton variant="discover" />
  {:else if error}
    <div class="alert alert-danger stack" role="alert">
      <p class="mb-0">{error}</p>
      <button class="btn btn-outline-danger align-self-start" onclick={() => load(true)}>Try again</button>
    </div>
  {:else if !card}
    <div class="sheet-card stack empty-state">
      <div class="empty-ball mx-auto" aria-hidden="true"></div>
      <h2>No one in range right now</h2>
      <p class="muted">Try a wider radius, add play times, or check your area.</p>
      <div class="d-flex justify-content-center gap-2">
        <button class="btn btn-primary" onclick={() => { radius = 30; load(true); }}>Widen radius</button>
        <a class="btn btn-outline-secondary" href="/onboarding">Change area</a>
      </div>
    </div>
  {:else}
    <article class="media-card" aria-labelledby={`name-${card.player.player_id}`}>
      <div class="media-frame">
        <div class="media-fallback" aria-hidden="true">
          <span>{initials(card.player.display_name)}</span>
        </div>
        {#if card.player.photo_urls.length}
          <img
            src={card.player.photo_urls[0]}
            alt={`${card.player.display_name}'s photo`}
            onerror={(event) => ((event.currentTarget as HTMLImageElement).hidden = true)}
          />
        {/if}
        <span class="chip media-chip">{distanceLabel(card.player.distance_miles)}</span>
        <button
          class="icon-button card-safety"
          type="button"
          onclick={() => openSafety(card, 'report')}
          aria-label={`Safety options for ${card.player.display_name}`}
        >
          <i class="bi bi-flag" aria-hidden="true"></i>
        </button>
      </div>

      <div class="media-body">
        <div class="name-row">
          <h2 id={`name-${card.player.player_id}`} class="mb-0">
            {card.player.display_name}, {card.player.age}
          </h2>
          <span class="chip level">{card.player.skill_level}</span>
        </div>

        <div class="week-strip" aria-hidden="true">
          {#each ['M', 'T', 'W', 'T', 'F', 'S', 'S'] as day, index}
            <span class:active={card.player.play_summary.toLowerCase().includes(['mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun'][index])}>{day}</span>
          {/each}
        </div>

        <dl class="meta">
          <div>
            <dt class="kicker">Plays</dt>
            <dd>{card.player.play_summary || 'No shared times listed'}</dd>
          </div>
          <div>
            <dt class="kicker">Distance</dt>
            <dd>{distanceLabel(card.player.distance_miles)}</dd>
          </div>
        </dl>

        {#if card.player.bio}<p class="bio clamp">{card.player.bio}</p>{/if}
        <a class="profile-link" href={`/discover/${card.player.player_id}?radius=${radius}`}>
          View full profile
        </a>

        {#if error}
          <p class="alert alert-danger py-2 mb-0" role="alert">{error}</p>
        {/if}

        {#if confirmPass === card.player.player_id}
          <div class="confirm-pass sticky-actions">
            <p class="small mb-2">You won't see each other again.</p>
            <div class="d-flex gap-2">
              <button class="btn btn-danger flex-grow-1" onclick={() => react(card.player.player_id, 'pass')} disabled={acting === card.player.player_id}>
                Not for me
              </button>
              <button class="btn btn-outline-secondary" onclick={() => (confirmPass = null)}>Cancel</button>
            </div>
          </div>
        {:else}
          <div class="card-actions sticky-actions">
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
    width: min(100% - (2 * var(--space-page-x)), 30rem);
    padding-block: var(--space-page-top) var(--space-page-bottom);
  }

  .discover-head {
    display: flex;
    flex-wrap: wrap;
    align-items: flex-end;
    justify-content: space-between;
    gap: 0.75rem;
  }

  .discover-head h1 {
    font-size: clamp(1.6rem, 4vw, 2rem);
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

  .level {
    text-transform: capitalize;
  }

  .profile-link {
    display: inline-flex;
    align-items: center;
    justify-self: start;
    min-height: var(--tap);
    font-weight: 700;
    text-decoration: underline;
    text-underline-offset: 0.18em;
  }

  .empty-ball {
    width: 4rem;
    height: 4rem;
    border-radius: 50%;
    background: radial-gradient(circle at 35% 30%, #f0ff9b, var(--ball) 55%, var(--ball-deep));
  }

  @media (max-width: 430px) {
    .discover-head {
      align-items: stretch;
      flex-direction: column;
    }

    .radius {
      width: 100%;
    }
  }
</style>
