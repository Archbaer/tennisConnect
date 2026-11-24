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
  <div class="row g-5 align-items-center">
    <div class="col-lg-6 hero-copy">
      <span class="chip optic mb-3"><i class="bi bi-geo-alt me-1" aria-hidden="true"></i> UK · Adults only</span>
      <h1>Find a real game,<br />not a group chat.</h1>
      <p class="lede">
        Meet compatible adult tennis players near you. Every photo is checked before anyone sees it.
        Your exact area never leaves the server.
      </p>
      <div class="d-flex flex-wrap gap-3">
        <a class="btn btn-primary btn-lg" href="#sign-in">
          <i class="bi bi-arrow-right" aria-hidden="true"></i> Find players
        </a>
        <a class="btn btn-outline-secondary btn-lg" href="#how">How safety works</a>
      </div>
      <div class="stat-strip mt-4" aria-label="Product facts">
        <span><strong>1</strong> approved photo</span>
        <span><strong>3</strong> whole-mile buckets</span>
        <span><strong>0</strong> exact locations shown</span>
      </div>
    </div>

    <div class="col-lg-6">
      <div class="court-card" aria-hidden="true">
        <div class="court-net"></div>
        <div class="court-line line-one"></div>
        <div class="court-line line-two"></div>
        <div class="hero-ball"></div>
        <div class="ball-shadow"></div>
        <div class="scoreboard">
          <span class="score">Tue, Thu</span>
          <span class="score">Evenings</span>
          <span class="score">3 miles</span>
        </div>
      </div>
    </div>
  </div>
</section>

<section class="page" id="how" aria-labelledby="how-title">
  <div class="d-flex align-items-center gap-2 mb-3">
    <span class="tennis-ball" aria-hidden="true"></span>
    <p class="score mb-0">Court side</p>
  </div>
  <h2 id="how-title" class="mb-4">Quiet until it is mutual.</h2>
  <div class="row g-4">
    <div class="col-md-4">
      <div class="surface h-100">
        <i class="bi bi-shield-check feature-icon" aria-hidden="true"></i>
        <h3>Photos checked first</h3>
        <p class="muted mb-0">Moderation runs before visibility. Rejected and pending photos are never public.</p>
      </div>
    </div>
    <div class="col-md-4">
      <div class="surface h-100">
        <i class="bi bi-hand-thumbs-up feature-icon" aria-hidden="true"></i>
        <h3>Two yeses make a match</h3>
        <p class="muted mb-0">A like is private. Instagram stays hidden until both people say yes.</p>
      </div>
    </div>
    <div class="col-md-4">
      <div class="surface h-100">
        <i class="bi bi-slash-circle feature-icon" aria-hidden="true"></i>
        <h3>Exit any time</h3>
        <p class="muted mb-0">Report or block. A block closes the match and stops new photo and handle access.</p>
      </div>
    </div>
  </div>
</section>

<section class="page narrow sign-in" id="sign-in" aria-labelledby="sign-in-title">
  <div class="surface">
    <div class="d-flex align-items-center gap-2 mb-2">
      <span class="tennis-ball" aria-hidden="true"></span>
      <p class="score mb-0">Members</p>
    </div>
    <h2 id="sign-in-title">Step on court</h2>
    <p class="muted">No password. We send a one-time link.</p>
    {#if error}
      <p class="alert-box" role="alert">{error}</p>
    {/if}
    <form class="stack" onsubmit={requestLink}>
      <label class="form-label">
        E-mail
        <input
          class="form-control"
          type="email"
          name="email"
          autocomplete="email"
          required
          bind:value={email}
          placeholder="you@example.com"
        />
      </label>
      <button class="btn btn-primary" type="submit" disabled={submitting || !$online}>
        {submitting ? 'Sending…' : 'Send sign-in link'}
      </button>
      <small>By continuing you confirm you are 18 or older.</small>
    </form>
  </div>
</section>

<style>
  .hero {
    padding-block: 3rem 4rem;
    animation: fade-up 320ms ease both;
  }

  .lede {
    font-size: 1.12rem;
    max-width: 34rem;
    color: var(--muted);
  }

  .stat-strip {
    display: flex;
    flex-wrap: wrap;
    gap: 0.75rem 1.5rem;
    padding-top: 1rem;
    border-top: 1px solid var(--line);
    color: var(--muted);
    font-size: 0.86rem;
  }

  .stat-strip strong {
    color: var(--ink);
    font-variant-numeric: tabular-nums;
  }

  .court-card {
    position: relative;
    min-height: 24rem;
    border-radius: 26px;
    background: linear-gradient(145deg, var(--court) 0%, var(--court-deep) 62%, #03191e 100%);
    overflow: hidden;
    box-shadow: var(--shadow);
    isolation: isolate;
  }

  .court-card::before {
    content: '';
    position: absolute;
    inset: 12% 9% 18%;
    border: 2px solid rgb(216 255 62 / 38%);
    transform: perspective(600px) rotateX(14deg);
  }

  .court-net {
    position: absolute;
    left: 9%;
    right: 9%;
    top: 47%;
    height: 2px;
    background: repeating-linear-gradient(to right, rgb(247 249 244 / 55%) 0 8px, transparent 8px 13px);
  }

  .court-line {
    position: absolute;
    border: 2px solid rgb(247 249 244 / 22%);
  }

  .line-one {
    inset: 20% 24% 28%;
    transform: perspective(500px) rotateX(18deg);
  }

  .line-two {
    left: 50%;
    top: 12%;
    bottom: 18%;
    width: 0;
    border-width: 0 0 0 2px;
  }

  .hero-ball {
    position: absolute;
    right: 17%;
    top: 18%;
    width: 4.4rem;
    aspect-ratio: 1;
    border-radius: 50%;
    background: radial-gradient(circle at 35% 30%, #f3ffb1, var(--ball) 55%, var(--ball-deep));
    box-shadow: inset -8px -10px 18px rgb(6 38 45 / 18%);
    animation: ball-bounce 2.8s ease-in-out infinite;
  }

  .ball-shadow {
    position: absolute;
    right: 19%;
    top: 34%;
    width: 3.4rem;
    height: 0.8rem;
    border-radius: 50%;
    background: rgb(0 0 0 / 28%);
    filter: blur(4px);
    animation: shadow-pulse 2.8s ease-in-out infinite;
  }

  .scoreboard {
    position: absolute;
    left: 1.25rem;
    bottom: 1.25rem;
    display: grid;
    gap: 0.5rem;
    padding: 0.9rem 1rem;
    border-radius: 14px;
    background: rgb(3 25 30 / 72%);
    border: 1px solid rgb(216 255 62 / 25%);
    color: var(--ball);
  }

  .feature-icon {
    display: inline-grid;
    place-items: center;
    width: 2.6rem;
    height: 2.6rem;
    margin-bottom: 0.75rem;
    border-radius: 50%;
    background: var(--ball);
    color: var(--court-deep);
    font-size: 1.25rem;
  }

  .sign-in {
    padding-bottom: 4rem;
  }

  @keyframes shadow-pulse {
    0%,
    100% {
      transform: scaleX(1);
      opacity: 0.4;
    }
    50% {
      transform: scaleX(0.7);
      opacity: 0.2;
    }
  }

  @media (max-width: 991px) {
    .court-card {
      min-height: 19rem;
    }
  }
</style>
