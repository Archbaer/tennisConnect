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
        <line x1="12" x2="408" y1="30" y2="30" />
        <line x1="12" x2="408" y1="75" y2="75" />
        <line x1="12" x2="408" y1="120" y2="120" />
        <path class="preview-area" d="M12,120 L12,98 L68,86 L124,102 L180,70 L236,78 L292,46 L348,58 L404,30 L408,30 L408,120 Z" />
        <path class="preview-line" d="M12,98 L68,86 L124,102 L180,70 L236,78 L292,46 L348,58 L404,30" />
        <circle cx="12" cy="98" r="2.5" />
        <circle cx="68" cy="86" r="2.5" />
        <circle cx="124" cy="102" r="2.5" />
        <circle cx="180" cy="70" r="2.5" />
        <circle cx="236" cy="78" r="2.5" />
        <circle cx="292" cy="46" r="2.5" />
        <circle cx="348" cy="58" r="2.5" />
        <circle cx="404" cy="30" r="2.5" />
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
    <article class="sheet-card feature-card">
      <i class="bi bi-shield-check feature-icon" aria-hidden="true"></i>
      <h3>Photos checked first</h3>
      <p class="muted mb-0">Moderation runs before visibility. Rejected and pending photos are never public.</p>
    </article>
    <article class="sheet-card feature-card">
      <i class="bi bi-hand-thumbs-up feature-icon" aria-hidden="true"></i>
      <h3>Two yeses make a match</h3>
      <p class="muted mb-0">A like is private. Instagram stays hidden until both people say yes.</p>
    </article>
    <article class="sheet-card feature-card">
      <i class="bi bi-slash-circle feature-icon" aria-hidden="true"></i>
      <h3>Exit any time</h3>
      <p class="muted mb-0">Report or block. A block closes the match and stops new photo and handle access.</p>
    </article>
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
    padding-block: 3rem 4rem;
    animation: fade-up 320ms ease both;
  }

  .hero-grid {
    display: grid;
    grid-template-columns: minmax(0, 1.05fr) minmax(0, 0.95fr);
    align-items: center;
    gap: 3rem;
  }

  .hero h1 {
    font-size: clamp(2.6rem, 6vw, 4.4rem);
    max-width: 12ch;
  }

  .lede {
    max-width: 34rem;
    color: var(--muted);
    font-size: 1.12rem;
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
    display: grid;
    gap: 0.75rem;
    padding: 1rem;
    border: 1px solid var(--line);
    border-radius: var(--radius-control);
    background: var(--surface);
    box-shadow: var(--shadow);
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
  }

  .preview-kpis div {
    display: grid;
    gap: 0.2rem;
    padding: 0.65rem;
    background: var(--surface);
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
    background: var(--paper);
  }

  .preview-chart line {
    stroke: var(--chart-grid);
    stroke-width: 1;
  }

  .preview-chart .preview-area {
    fill: var(--chart-fill);
  }

  .preview-chart .preview-line {
    fill: none;
    stroke: var(--chart-line);
    stroke-width: 2;
  }

  .preview-chart circle {
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
    gap: 2px;
    height: 6px;
  }

  .preview-segments i {
    flex: 1;
    border-radius: 999px;
    background: var(--line);
  }

  .preview-segments i.on {
    background: var(--tracker-progress);
  }

  .section-intro {
    margin-bottom: 1.25rem;
  }

  .section-intro h2 {
    margin: 0;
  }

  .feature-grid {
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 1rem;
  }

  .feature-card {
    display: grid;
    align-content: start;
    gap: 0.35rem;
  }

  .feature-card h3 {
    margin-bottom: 0.15rem;
  }

  .feature-icon {
    display: inline-grid;
    place-items: center;
    width: 2.6rem;
    height: 2.6rem;
    margin-bottom: 0.5rem;
    border-radius: 50%;
    background: var(--ball);
    color: var(--court-deep);
    font-size: 1.25rem;
  }

  .sign-in {
    padding-bottom: 4rem;
  }

  @media (max-width: 991px) {
    .hero-grid,
    .feature-grid {
      grid-template-columns: minmax(0, 1fr);
    }

    .hero-grid {
      gap: 1.5rem;
    }

    .hero {
      padding-block: 2rem 3rem;
    }

    .preview-shell {
      max-width: 34rem;
    }
  }

  @media (max-width: 480px) {
    .preview-progress > div {
      grid-template-columns: 6rem minmax(0, 1fr) 1.7rem;
    }
  }
</style>
