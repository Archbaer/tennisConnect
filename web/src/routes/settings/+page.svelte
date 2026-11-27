<script lang="ts">
  import { goto } from '$app/navigation';
  import { apiJSON } from '#lib/api';
  import AreaSheet from '#lib/AreaSheet.svelte';
  import Disclosure from '#lib/Disclosure.svelte';
  import LoadingSkeleton from '#lib/LoadingSkeleton.svelte';
  import MatchingSheet from '#lib/MatchingSheet.svelte';
  import { session } from '#lib/session';

  let areaSheet: { open: () => Promise<void> };
  let matchingSheet: { open: () => Promise<void> };
  let status = '';

  async function signOut() {
    try {
      await apiJSON('/api/v1/auth/sign-out', { method: 'POST' });
    } finally {
      await goto('/');
    }
  }

  function areaSaved(label: string) {
    status = `Area saved: ${label}.`;
  }

  function matchingSaved() {
    status = 'Matching preferences saved.';
  }
</script>

<svelte:head>
  <title>Settings · Tennis Connect</title>
</svelte:head>

<section class="page settings-page stack">
  <header class="page-head">
    <div>
      <p class="kicker">Account</p>
      <h1>Settings</h1>
      <p class="muted mb-0">Your card, your boundaries and your data.</p>
    </div>
  </header>

  <p class="visually-hidden" role="status" aria-live="polite">{status}</p>

  {#if $session === 'unknown'}
    <LoadingSkeleton variant="settings" />
  {:else}
    <div class="settings-grid">
    <section class="sheet-card settings-card" aria-labelledby="settings-card-title">
      <h2 id="settings-card-title"><i class="bi bi-person-vcard me-2" aria-hidden="true"></i>Your card</h2>
      <a class="settings-row" href="/profile/edit">
        <span>Edit profile</span>
        <i class="bi bi-chevron-right settings-chevron" aria-hidden="true"></i>
      </a>
      <a class="settings-row" href="/profile/play-times">
        <span>Edit play times</span>
        <i class="bi bi-chevron-right settings-chevron" aria-hidden="true"></i>
      </a>
      <a class="settings-row" href="/profile/photos">
        <span>Manage photos</span>
        <i class="bi bi-chevron-right settings-chevron" aria-hidden="true"></i>
      </a>
      <button class="settings-row" type="button" onclick={() => areaSheet.open()}>
        <span>Area and discovery radius</span>
        <i class="bi bi-chevron-right settings-chevron" aria-hidden="true"></i>
      </button>
    </section>

    <section class="sheet-card settings-card" aria-labelledby="settings-matching-title">
      <h2 id="settings-matching-title"><i class="bi bi-people me-2" aria-hidden="true"></i>Matching</h2>
      <button class="settings-row" type="button" onclick={() => matchingSheet.open()}>
        <span>Who you'd like to meet</span>
        <i class="bi bi-chevron-right settings-chevron" aria-hidden="true"></i>
      </button>
      <a class="settings-row" href="/profile">
        <span>Preview as another player</span>
        <i class="bi bi-chevron-right settings-chevron" aria-hidden="true"></i>
      </a>
    </section>

    <Disclosure class="settings-disclosure" label="Privacy and safety" level={2} icon="bi-shield-check">
      <p class="muted">
        Your exact area, e-mail and date of birth are never returned by the API. Other players see a
        coarse area only through distance buckets. Instagram is disclosed only inside a live match.
      </p>
      <p class="muted mb-0">
        To report or block someone, use the safety button on their Discover card or in your match.
      </p>
    </Disclosure>

    <section class="sheet-card settings-card" aria-labelledby="settings-data-title">
      <h2 id="settings-data-title"><i class="bi bi-database me-2" aria-hidden="true"></i>Your data</h2>
      <a class="settings-row" href="/account/export">
        <span>Download my data</span>
        <i class="bi bi-chevron-right settings-chevron" aria-hidden="true"></i>
      </a>
      <a class="settings-row" href="/account/delete">
        <span>Delete account</span>
        <i class="bi bi-chevron-right settings-chevron" aria-hidden="true"></i>
      </a>
    </section>

    <section class="sheet-card settings-card" aria-labelledby="settings-about-title">
      <h2 id="settings-about-title"><i class="bi bi-info-circle me-2" aria-hidden="true"></i>About</h2>
      <a class="settings-row" href="/#how">
        <span>How we keep you safe</span>
        <i class="bi bi-chevron-right settings-chevron" aria-hidden="true"></i>
      </a>
      <p class="settings-note">E-mail is used for sign-in only. It is never shown to another player.</p>
    </section>

    <section class="sheet-card settings-card" aria-labelledby="settings-session-title">
      <h2 id="settings-session-title"><i class="bi bi-box-arrow-right me-2" aria-hidden="true"></i>Session</h2>
      <button class="settings-row" type="button" onclick={signOut}>
        <span>Sign out</span>
        <i class="bi bi-chevron-right settings-chevron" aria-hidden="true"></i>
      </button>
    </section>
    </div>
  {/if}

  <AreaSheet bind:this={areaSheet} onSaved={areaSaved} announceToast={false} />
  <MatchingSheet bind:this={matchingSheet} onSaved={matchingSaved} />
</section>

<style>
  .settings-page {
    width: min(100% - (2 * var(--space-page-x)), 48rem);
  }

  .settings-grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    align-items: start;
    gap: var(--space-stack);
  }

  .settings-card {
    padding: calc(var(--space-card) * 0.9) var(--space-card);
  }

  :global(.settings-disclosure) {
    font-size: 0.86rem;
  }

  :global(.settings-disclosure .disclosure-title) {
    font-size: 1.05rem;
  }

  .settings-card h2 {
    margin-bottom: 0.55rem;
    font-size: 1.1rem;
  }

  .settings-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;
    min-height: var(--tap);
    padding: 0.55rem 0.35rem;
    border: 0;
    border-top: 1px solid var(--line);
    border-radius: 10px;
    background: transparent;
    color: var(--ink);
    font: inherit;
    font-weight: 600;
    text-align: left;
    text-decoration: none;
    cursor: pointer;
    transition: background 160ms ease, color 160ms ease, transform 160ms ease, box-shadow 160ms ease;
  }

  .settings-row:hover {
    background: var(--surface-hover);
    color: var(--court);
    transform: translateX(2px);
    box-shadow: inset 3px 0 0 var(--court-light);
  }

  .settings-chevron {
    color: var(--muted);
    font-size: 0.75rem;
  }

  .settings-note {
    margin: 0.75rem 0 0;
    color: var(--muted);
    font-size: 0.86rem;
  }

  @media (min-width: 768px) {
    .settings-grid {
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 1rem;
    }
  }
</style>
