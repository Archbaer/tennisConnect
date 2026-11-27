<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { goto } from '$app/navigation';
  import { apiJSON, type Match } from '#lib/api';
  import { noteMatchRevealed } from '#lib/matchState';

  let matchID = '';
  let match: Match | null = null;
  let loading = true;
  let error = '';
  let heading: HTMLHeadingElement;

  onMount(async () => {
    matchID = window.location.pathname.split('/').filter(Boolean)[1] ?? '';
    try {
      match = await apiJSON<Match>(`/api/v1/matches/${matchID}`);
      if (match.revealed) {
        await goto(`/matches/${matchID}`, { replace: true });
        return;
      }
      await apiJSON(`/api/v1/matches/${matchID}/reveal`, { method: 'POST' });
      noteMatchRevealed();
      await tick();
      setTimeout(() => heading?.focus(), 50);
    } catch {
      error = 'This match is no longer available.';
    } finally {
      loading = false;
    }
  });
</script>

<svelte:head>
  <title>It's a match · Tennis Connect</title>
</svelte:head>

<section class="reveal-page">
  {#if loading}
    <p role="status">Opening your match…</p>
  {:else if error || !match}
    <div class="reveal-card stack">
      <h1>This match is closed</h1>
      <p class="muted">It may have ended or the other player is no longer available.</p>
      <a class="btn btn-primary align-self-start" href="/matches">Back to matches</a>
    </div>
  {:else}
    <div class="reveal-card">
      <div class="flood" aria-hidden="true"></div>
      <div class="reveal-copy">
        <p class="kicker mb-2">It's a match</p>
        <h1 id="reveal-title" tabindex="-1" bind:this={heading}>You're both in.</h1>
        <p class="reveal-sub">You and {match.display_name} both said yes to a game.</p>
        <div class="reveal-stats" aria-hidden="true">
          <span>Match confirmed</span>
          <span>Instagram unlocks</span>
          <span>Meet on court</span>
        </div>
        <div class="reveal-actions">
          <a class="btn btn-primary" href={`/matches/${match.match_id}`}>See match</a>
          <button class="btn btn-outline-light" type="button" onclick={() => goto('/discover')}>Keep looking</button>
        </div>
      </div>
    </div>
    <p class="visually-hidden" role="status" aria-live="polite">
      It's a match. You and {match.display_name} both said yes. Instagram is now visible in this match.
    </p>
  {/if}
</section>

<style>
  .reveal-page {
    display: grid;
    place-items: center;
    min-height: calc(100dvh - 4.5rem);
    padding: 1rem;
    background: var(--court-deep);
  }

  .reveal-card {
    position: relative;
    display: grid;
    align-content: center;
    min-height: 70dvh;
    width: min(100%, 30rem);
    overflow: hidden;
    border-radius: var(--radius-control);
    background: linear-gradient(165deg, #0e5a3a, var(--court-deep) 62%);
    color: white;
    padding: 2rem 1.4rem;
    box-shadow: 0 28px 80px rgb(0 0 0 / 28%);
  }

  .flood {
    position: absolute;
    right: -5rem;
    top: -5rem;
    width: 18rem;
    height: 18rem;
    border-radius: 50%;
    background: radial-gradient(circle, var(--ball) 0 42%, transparent 68%);
    opacity: 0.88;
    animation: match-flood 420ms cubic-bezier(0.2, 0.8, 0.2, 1);
  }

  .reveal-copy {
    position: relative;
    z-index: 1;
    display: grid;
    gap: 0.75rem;
  }

  .reveal-copy h1:focus {
    outline: 3px solid var(--sky);
    outline-offset: 4px;
  }

  .reveal-copy h1 {
    color: white;
    font-size: clamp(2.6rem, 12vw, 4rem);
    max-width: 8ch;
  }

  .reveal-copy .kicker {
    color: var(--ball);
  }

  .reveal-sub {
    color: #dce8e2;
    font-size: 1.1rem;
    font-weight: 600;
    max-width: 24ch;
  }

  .reveal-stats {
    display: flex;
    flex-wrap: wrap;
    gap: 0.4rem;
    padding-block: 0.25rem;
  }

  .reveal-stats span {
    padding: 0.2rem 0.6rem;
    border: 1px solid rgb(255 255 255 / 28%);
    border-radius: 999px;
    color: #dce8e2;
    font-family: var(--font-mono);
    font-size: 0.66rem;
    letter-spacing: 0.04em;
    text-transform: uppercase;
  }

  .reveal-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    padding-top: 0.5rem;
  }

  @media (max-width: 520px) {
    .reveal-page {
      align-items: center;
    }
  }
</style>
