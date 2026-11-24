<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { apiJSON } from '#lib/api';
  import { online } from '#lib/offline';
  import { loadSession } from '#lib/session';

  let email = '';
  let error = '';
  let submitting = false;

  onMount(async () => {
    if ((await loadSession()) === 'signed-in') {
      await goto('/discover');
    }
  });

  async function requestLink(event: SubmitEvent) {
    event.preventDefault();
    error = '';
    submitting = true;
    try {
      await apiJSON('/api/v1/auth/link', {
        method: 'POST',
        body: JSON.stringify({ email })
      });
      await goto('/check-inbox');
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'We could not send that link.';
    } finally {
      submitting = false;
    }
  }
</script>

<svelte:head>
  <title>Tennis Connect · Play more tennis</title>
</svelte:head>

<section class="hero page">
  <div class="hero-copy">
    <p class="score">UK · Adults only</p>
    <h1>Find a real game, not a group chat.</h1>
    <p class="lede">
      Meet compatible adult tennis players near you. Every photo is checked before anyone sees it.
      Your exact area is never shown.
    </p>
    <div class="row">
      <a class="button" href="#sign-in">Find players</a>
      <a class="button secondary" href="#how">How safety works</a>
    </div>
  </div>

  <div class="hero-art" aria-hidden="true">
    <div class="court-line line-one"></div>
    <div class="court-line line-two"></div>
    <div class="ball"></div>
    <p class="score">Tue, Thu evenings<br />3 miles<br />Intermediate</p>
  </div>
</section>

<section class="page grid two" aria-labelledby="how">
  <div class="surface">
    <h2 id="how">One introduction at a time</h2>
    <p>
      Discovery shows who shares your level, your week and your area. A like is private. When two
      people like each other, the match is mutual and Instagram can be shared.
    </p>
  </div>
  <div class="surface">
    <h2>Built to keep strangers safe</h2>
    <p>
      Photos are moderated before visibility. Contact details are blocked in names and bios. You can
      report or block at any point, and a banned account cannot return.
    </p>
  </div>
</section>

<section class="page narrow sign-in" id="sign-in" aria-labelledby="sign-in-title">
  <div class="surface stack">
    <div>
      <p class="score">Members</p>
      <h2 id="sign-in-title">Sign in with e-mail</h2>
      <p class="muted">No password. We send a one-time link.</p>
    </div>
    {#if error}
      <p class="alert-box" role="alert">{error}</p>
    {/if}
    <form class="stack" onsubmit={requestLink}>
      <label>
        E-mail
        <input
          type="email"
          name="email"
          autocomplete="email"
          required
          bind:value={email}
          placeholder="you@example.com"
        />
      </label>
      <button type="submit" disabled={submitting || !$online}>
        {submitting ? 'Sending…' : 'Send sign-in link'}
      </button>
      <small>By continuing you confirm you are 18 or older.</small>
    </form>
  </div>
</section>

<style>
  .hero {
    display: grid;
    gap: 2rem;
    align-items: center;
    padding-block: 2.5rem 3rem;
  }

  .lede {
    font-size: 1.1rem;
    max-width: 34rem;
    color: var(--muted);
  }

  .hero-art {
    position: relative;
    min-height: 18rem;
    border-radius: var(--radius);
    background: var(--court);
    overflow: hidden;
    display: grid;
    place-items: end start;
    padding: 1.25rem;
  }

  .hero-art .score {
    color: var(--optic);
    line-height: 1.7;
    position: relative;
    z-index: 2;
  }

  .court-line {
    position: absolute;
    border: 2px solid rgb(220 242 63 / 55%);
  }

  .line-one {
    inset: 2rem 2rem 5rem 4rem;
    transform: skewY(-8deg);
  }

  .line-two {
    inset: 5rem auto 2rem 1.5rem;
    width: 60%;
    transform: rotate(-8deg);
  }

  .ball {
    position: absolute;
    right: 3rem;
    top: 2.5rem;
    width: 4.5rem;
    aspect-ratio: 1;
    border-radius: 50%;
    background: var(--optic);
    box-shadow: 0 0 0 10px rgb(220 242 63 / 14%);
  }

  .sign-in {
    padding-bottom: 4rem;
  }

  @media (min-width: 820px) {
    .hero {
      grid-template-columns: 1.2fr 1fr;
      min-height: 30rem;
    }
  }
</style>
