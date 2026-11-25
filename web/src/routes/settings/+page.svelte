<script lang="ts">
  import { goto } from '$app/navigation';
  import { apiJSON } from '#lib/api';
  import AreaSheet from '#lib/AreaSheet.svelte';
  import MatchingSheet from '#lib/MatchingSheet.svelte';

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
  <header class="settings-head">
    <p class="score mb-1"><span class="tennis-ball me-2" aria-hidden="true"></span>Account</p>
    <h1>Settings</h1>
    <p class="muted mb-0">Your card, your boundaries and your data.</p>
  </header>

  <p class="visually-hidden" role="status" aria-live="polite">{status}</p>

  <section class="settings-card" aria-labelledby="settings-card-title">
    <h2 id="settings-card-title"><i class="bi bi-person-vcard me-2" aria-hidden="true"></i>Your card</h2>
    <a class="settings-row" href="/profile/edit">Edit profile</a>
    <a class="settings-row" href="/profile/play-times">Edit play times</a>
    <a class="settings-row" href="/profile/photos">Manage photos</a>
    <button class="settings-row" type="button" onclick={() => areaSheet.open()}>Area and discovery radius</button>
  </section>

  <section class="settings-card" aria-labelledby="settings-matching-title">
    <h2 id="settings-matching-title"><i class="bi bi-people me-2" aria-hidden="true"></i>Matching</h2>
    <button class="settings-row" type="button" onclick={() => matchingSheet.open()}>
      Who you'd like to meet
    </button>
    <a class="settings-row" href="/profile">Preview as another player</a>
  </section>

  <section class="settings-card" aria-labelledby="settings-safety-title">
    <h2 id="settings-safety-title"><i class="bi bi-shield-check me-2" aria-hidden="true"></i>Privacy and safety</h2>
    <p class="muted">
      Your exact area, e-mail and date of birth are never returned by the API. Other players see a
      coarse area only through distance buckets. Instagram is disclosed only inside a live match.
    </p>
    <p class="muted mb-0">
      To report or block someone, use the safety button on their Discover card or in your match.
    </p>
  </section>

  <section class="settings-card" aria-labelledby="settings-data-title">
    <h2 id="settings-data-title"><i class="bi bi-database me-2" aria-hidden="true"></i>Your data</h2>
    <a class="settings-row" href="/account/export">Download my data</a>
    <a class="settings-row" href="/account/delete">Delete account</a>
  </section>

  <section class="settings-card" aria-labelledby="settings-about-title">
    <h2 id="settings-about-title"><i class="bi bi-info-circle me-2" aria-hidden="true"></i>About</h2>
    <a class="settings-row" href="/#how">How we keep you safe</a>
    <p class="settings-note">E-mail is used for sign-in only. It is never shown to another player.</p>
  </section>

  <section class="settings-card" aria-labelledby="settings-session-title">
    <h2 id="settings-session-title"><i class="bi bi-box-arrow-right me-2" aria-hidden="true"></i>Session</h2>
    <button class="settings-row" type="button" onclick={signOut}>Sign out</button>
  </section>

  <AreaSheet bind:this={areaSheet} onSaved={areaSaved} />
  <MatchingSheet bind:this={matchingSheet} onSaved={matchingSaved} />
</section>

<style>
  .settings-page {
    width: min(100% - 1.5rem, 36rem);
  }

  .settings-head {
    text-align: center;
    padding-block: 0.5rem 0.25rem;
  }

  .settings-head h1 {
    font-size: clamp(2rem, 8vw, 2.8rem);
  }

  .settings-card {
    border: 1px solid var(--line);
    border-radius: var(--radius);
    background: var(--surface);
    box-shadow: var(--shadow-small);
    padding: 1.15rem;
    text-align: center;
  }

  .settings-card h2 {
    font-size: 1.15rem;
    margin-bottom: 0.75rem;
  }

  .settings-card .muted {
    text-align: left;
  }

  .settings-row {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 100%;
    min-height: 3rem;
    padding: 0.75rem 0.5rem;
    border: 0;
    border-top: 1px solid var(--line);
    border-radius: 0;
    background: transparent;
    color: var(--court);
    font: inherit;
    font-weight: 600;
    text-align: center;
    text-decoration: none;
    cursor: pointer;
  }

  .settings-row:hover {
    background: #eef4ef;
    color: var(--court-deep);
    transform: none;
    box-shadow: none;
  }

  .settings-row:last-child {
    border-bottom: 0;
  }

  .settings-note {
    margin: 0.75rem 0 0;
    color: var(--muted);
    font-size: 0.86rem;
  }
</style>
