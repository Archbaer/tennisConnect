<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { apiJSON } from '#lib/api';
  import Disclosure from '#lib/Disclosure.svelte';
  import { online } from '#lib/offline';
  import { loadSession } from '#lib/session';
  import { showToast } from '#lib/toasts';

  let email = '';
  let error = '';
  let submitting = false;

  const previewPoints = [
    { x: 12, y: 98 },
    { x: 68, y: 86 },
    { x: 124, y: 102 },
    { x: 180, y: 70 },
    { x: 236, y: 78 },
    { x: 292, y: 46 },
    { x: 348, y: 58 },
    { x: 404, y: 30 }
  ];

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
      showToast('Sign-in link sent. Check your inbox.');
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
  <div class="hero-grid">
    <div class="hero-copy">
      <span class="chip optic mb-3"><i class="bi bi-geo-alt me-1" aria-hidden="true"></i> UK · Adults only</span>
      <h1>Find a real game,<br />not a group chat.</h1>
      <p class="lede">
        Meet compatible adult tennis players near you. Every photo is checked before anyone sees it.
        Your exact area never leaves the server.
      </p>
      <div class="hero-actions">
        <a class="btn btn-primary btn-lg" href="#sign-in">
          <i class="bi bi-arrow-right" aria-hidden="true"></i> Find players
        </a>
        <a class="btn btn-outline-secondary btn-lg" href="#how">How safety works</a>
      </div>

      <section class="kpi-strip three landing-kpis" aria-label="Product facts">
        <article class="kpi">
          <p class="kpi-label">Approved photo</p>
          <p class="kpi-value">1</p>
          <p class="kpi-delta">before you appear</p>
        </article>
        <article class="kpi">
          <p class="kpi-label">Distance buckets</p>
          <p class="kpi-value">3</p>
          <p class="kpi-delta">whole-mile bands</p>
        </article>
        <article class="kpi">
          <p class="kpi-label">Exact locations</p>
          <p class="kpi-value">0</p>
          <p class="kpi-delta">ever shown</p>
        </article>
      </section>
    </div>

    <div class="preview-shell" aria-hidden="true">
      <div class="preview-bar">
        <span class="preview-brand"><span class="tennis-ball"></span> Tennis Connect</span>
        <span class="preview-chip">Overview</span>
      </div>
      <div class="preview-kpis">
        <div><span>Matches</span><strong>12</strong></div>
        <div><span>Win rate</span><strong>62%</strong></div>
        <div><span>This month</span><strong>4</strong></div>
      </div>
      <svg class="preview-chart" viewBox="0 0 420 150" role="presentation">
        <defs>
          <linearGradient id="preview-fill" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" class="preview-stop-start" />
            <stop offset="100%" class="preview-stop-end" />
          </linearGradient>
          <linearGradient id="preview-line" x1="0" y1="0" x2="1" y2="0">
            <stop offset="0%" class="preview-line-stop-start" />
            <stop offset="100%" class="preview-line-stop-end" />
          </linearGradient>
        </defs>
        <line x1="12" x2="408" y1="30" y2="30" />
        <line x1="12" x2="408" y1="75" y2="75" />
        <line x1="12" x2="408" y1="120" y2="120" />
        <path
          class="preview-area"
          fill="url(#preview-fill)"
          d="M12,98 C31,98 49,86 68,86 C87,86 105,102 124,102 C143,102 161,70 180,70 C199,70 217,78 236,78 C255,78 273,46 292,46 C311,46 329,58 348,58 C367,58 385,30 404,30 L408,30 L408,120 L12,120 Z"
        />
        <path
          class="preview-line-halo"
          d="M12,98 C31,98 49,86 68,86 C87,86 105,102 124,102 C143,102 161,70 180,70 C199,70 217,78 236,78 C255,78 273,46 292,46 C311,46 329,58 348,58 C367,58 385,30 404,30"
        />
        <path
          class="preview-line"
          stroke="url(#preview-line)"
          d="M12,98 C31,98 49,86 68,86 C87,86 105,102 124,102 C143,102 161,70 180,70 C199,70 217,78 236,78 C255,78 273,46 292,46 C311,46 329,58 348,58 C367,58 385,30 404,30"
        />
        {#each previewPoints as point}
          <circle class="preview-halo" cx={point.x} cy={point.y} r="6" />
          <circle class="preview-dot" cx={point.x} cy={point.y} r="2.5" />
        {/each}
      </svg>
      <div class="preview-progress">
        <div>
          <span>Tue evening</span>
          <span class="preview-segments"><i></i><i></i><i></i><i></i><i class="on"></i><i class="on"></i><i class="on"></i><i class="on"></i><i class="on"></i><i class="on"></i></span>
          <span>3h</span>
        </div>
        <div>
          <span>Thu evening</span>
          <span class="preview-segments"><i></i><i></i><i></i><i class="on"></i><i class="on"></i><i class="on"></i><i class="on"></i><i class="on"></i><i class="on"></i><i class="on"></i></span>
          <span>2h</span>
        </div>
      </div>
    </div>
  </div>
</section>

<section class="page" id="how" aria-labelledby="how-title">
  <div class="section-intro">
    <p class="kicker">Court side</p>
    <h2 id="how-title">Quiet until it is mutual.</h2>
  </div>
  <div class="feature-grid">
    <Disclosure class="feature-card" label="Photos checked first" icon="bi-shield-check">
      <p class="muted mb-0">Moderation runs before visibility. Rejected and pending photos are never public.</p>
    </Disclosure>
    <Disclosure class="feature-card" label="Two yeses make a match" icon="bi-hand-thumbs-up">
      <p class="muted mb-0">A like is private. Instagram stays hidden until both people say yes.</p>
    </Disclosure>
    <Disclosure class="feature-card" label="Exit any time" icon="bi-slash-circle">
      <p class="muted mb-0">Report or block. A block closes the match and stops new photo and handle access.</p>
    </Disclosure>
  </div>
</section>

<section class="page narrow sign-in" id="sign-in" aria-labelledby="sign-in-title">
  <div class="sheet-card">
    <div class="d-flex align-items-center gap-2 mb-2">
      <span class="tennis-ball" aria-hidden="true"></span>
      <p class="kicker mb-0">Members</p>
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
    padding-block: 1.1rem 1.75rem;
    animation: fade-up 320ms ease both;
  }

  .hero-grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    align-items: center;
    gap: 1.25rem;
  }

  .hero h1 {
    font-size: clamp(2rem, 10vw, 2.9rem);
    max-width: 12ch;
  }

  .lede {
    max-width: 34rem;
    color: var(--muted);
    font-size: var(--type-lede);
  }

  .hero-actions {
    display: flex;
    flex-wrap: wrap;
    gap: 0.6rem;
  }

  .landing-kpis {
    margin-top: 1.5rem;
  }

  .preview-shell {
    display: none;
    gap: 0.75rem;
    padding: 1rem;
    border: 1px solid var(--line);
    border-radius: var(--radius-control);
    background: linear-gradient(180deg, var(--surface-top), var(--surface) 72%);
    box-shadow: var(--card-highlight), var(--tc-shadow-3);
  }

  .preview-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    padding-bottom: 0.7rem;
    border-bottom: 1px solid var(--line);
  }

  .preview-brand {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    font-size: 0.82rem;
    font-weight: 800;
    letter-spacing: -0.02em;
  }

  .preview-chip {
    padding: 0.15rem 0.55rem;
    border-radius: 999px;
    background: var(--chip-bg);
    color: var(--muted);
    font-size: 0.68rem;
    font-weight: 700;
  }

  .preview-kpis {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 1px;
    overflow: hidden;
    border: 1px solid var(--line);
    border-radius: var(--radius-control);
    background: var(--line);
    box-shadow: var(--card-highlight), var(--tc-shadow-1);
  }

  .preview-kpis div {
    display: grid;
    gap: 0.2rem;
    padding: 0.65rem;
    background: linear-gradient(180deg, var(--surface-top), var(--surface));
  }

  .preview-kpis span {
    color: var(--muted);
    font-family: var(--font-mono);
    font-size: 0.58rem;
    font-weight: 700;
    letter-spacing: 0.05em;
    text-transform: uppercase;
  }

  .preview-kpis strong {
    font-size: 1.35rem;
    letter-spacing: -0.04em;
    line-height: 1;
  }

  .preview-chart {
    display: block;
    width: 100%;
    height: auto;
    border: 1px solid var(--line);
    border-radius: var(--radius-control);
    background: linear-gradient(180deg, var(--surface-2), var(--paper));
    box-shadow: inset 0 1px 0 var(--highlight);
  }

  .preview-chart line {
    stroke: var(--chart-grid);
    stroke-width: 1;
  }

  .preview-stop-start {
    stop-color: var(--chart-fill-start);
    stop-opacity: 0.34;
  }

  .preview-stop-end {
    stop-color: var(--chart-fill-start);
    stop-opacity: 0;
  }

  .preview-line-stop-start {
    stop-color: var(--chart-line-start);
  }

  .preview-line-stop-end {
    stop-color: var(--chart-line-end);
  }

  .preview-line-halo {
    fill: none;
    stroke: var(--chart-line-halo);
    stroke-width: 5;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .preview-chart .preview-line {
    fill: none;
    stroke-width: 2.5;
    stroke-linecap: round;
    stroke-linejoin: round;
    filter: drop-shadow(0 0 6px var(--chart-glow));
  }

  .preview-halo {
    fill: var(--chart-line-end);
    opacity: 0.14;
  }

  .preview-dot {
    fill: var(--surface);
    stroke: var(--chart-line);
    stroke-width: 2;
  }

  .preview-progress {
    display: grid;
    gap: 0.55rem;
  }

  .preview-progress > div {
    display: grid;
    grid-template-columns: 7rem minmax(0, 1fr) 2rem;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.72rem;
  }

  .preview-progress > div > span:first-child {
    color: var(--muted);
  }

  .preview-progress > div > span:last-child {
    color: var(--muted);
    font-family: var(--font-mono);
    text-align: right;
  }

  .preview-segments {
    display: flex;
    gap: 3px;
    height: 7px;
  }

  .preview-segments i {
    flex: 1;
    border-radius: 999px;
    background: var(--tracker-track);
  }

  .preview-segments i.on {
    background: linear-gradient(90deg, var(--tracker-from), var(--tracker-to));
    box-shadow: inset 0 1px 0 var(--highlight-soft);
  }

  .section-intro {
    margin-bottom: 1.25rem;
  }

  .section-intro h2 {
    margin: 0;
  }

  .feature-grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: var(--space-stack);
  }

  :global(.feature-card .disclosure-title) {
    display: flex;
    align-items: center;
    gap: 0.65rem;
    font-size: var(--type-h3);
  }

  :global(.feature-card .disclosure-title .bi) {
    display: inline-grid;
    place-items: center;
    flex: 0 0 auto;
    width: 2.25rem;
    height: 2.25rem;
    border-radius: 50%;
    background: var(--ball-face);
    color: var(--ball-ink);
    font-size: 1.05rem;
    box-shadow: inset 0 1px 0 var(--highlight-strong), 0 8px 18px var(--ball-glow);
  }

  .sign-in {
    padding-bottom: 1.5rem;
  }

  @media (min-width: 768px) {
    .preview-shell {
      display: grid;
      max-width: 34rem;
    }
  }

  @media (min-width: 992px) {
    .hero {
      padding-block: 2.5rem 3rem;
    }

    .hero-grid {
      grid-template-columns: minmax(0, 1.05fr) minmax(0, 0.95fr);
      gap: 2.5rem;
    }

    .hero h1 {
      font-size: clamp(3rem, 5vw, 4.4rem);
    }

    .feature-grid {
      grid-template-columns: repeat(3, minmax(0, 1fr));
      gap: 1rem;
    }

    .sign-in {
      padding-bottom: 3rem;
    }
  }

  @media (max-width: 480px) {
    .preview-progress > div {
      grid-template-columns: 6rem minmax(0, 1fr) 1.7rem;
    }
  }
</style>
