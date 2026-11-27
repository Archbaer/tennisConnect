<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON, formatDate, type Match, type Profile } from '#lib/api';
  import LoadingSkeleton from '#lib/LoadingSkeleton.svelte';
  import {
    activitySummary,
    buildDashboardMatches,
    buildKpis,
    chartGeometry,
    filterMatches,
    monthlyActivity,
    playWindows,
    type DashboardMatch,
    type Kpi,
    type MonthActivity,
    type PlayWindow
  } from '#lib/dashboard';

  let rawMatches: Match[] = [];
  let profile: Profile | null = null;
  let loading = true;
  let error = '';
  let query = '';
  let resultFilter: 'all' | 'W' | 'L' = 'all';
  let period: '12' | '6' = '12';
  let chart = chartGeometry([0]);
  let monthly: MonthActivity[] = [];
  let summary = '';
  let windows: { totalHours: number; windows: PlayWindow[] } = { totalHours: 0, windows: [] };

  const segments = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9];

  let matches: DashboardMatch[] = [];
  let kpis: Kpi[] = [];
  let filtered: DashboardMatch[] = [];
  let activeIndex = -1;

  $: matches = buildDashboardMatches(rawMatches);
  $: kpis = buildKpis(matches);
  $: monthly = monthlyActivity(matches).slice(period === '6' ? -6 : 0);
  $: chart = chartGeometry(monthly.map((month) => month.count));
  $: summary = activitySummary(monthly);
  $: windows = playWindows(profile?.availability ?? []);
  $: filtered = filterMatches(matches, query, resultFilter);
  $: activeMonth = activeIndex >= 0 ? monthly[activeIndex] : undefined;
  $: inspectedMonth = activeMonth ?? monthly[0];
  $: inspectedText = inspectedMonth
    ? `${inspectedMonth.label}: ${inspectedMonth.count} ${inspectedMonth.count === 1 ? 'match' : 'matches'}`
    : 'No months available';
  $: activePoint = activeIndex >= 0 ? chart.points[activeIndex] : undefined;
  $: tooltip = activePoint ? tooltipFor(activePoint) : undefined;

  function tooltipFor(point: { x: number; y: number }) {
    const half = 58;
    return {
      x: Math.min(Math.max(point.x, chart.left + half), chart.width - chart.right - half),
      y: point.y > 76 ? point.y - 14 : point.y + 14,
      below: point.y <= 76
    };
  }

  function inspectChart(event: KeyboardEvent) {
    const last = monthly.length - 1;
    if (last < 0) return;
    if (event.key === 'ArrowRight' || event.key === 'ArrowDown') {
      event.preventDefault();
      activeIndex = activeIndex < 0 ? 0 : Math.min(activeIndex + 1, last);
    } else if (event.key === 'ArrowLeft' || event.key === 'ArrowUp') {
      event.preventDefault();
      activeIndex = activeIndex < 0 ? last : Math.max(activeIndex - 1, 0);
    } else if (event.key === 'Home') {
      event.preventDefault();
      activeIndex = 0;
    } else if (event.key === 'End') {
      event.preventDefault();
      activeIndex = last;
    }
  }

  onMount(async () => {
    const [matchesResult, profileResult] = await Promise.allSettled([
      apiJSON<{ matches: Match[] }>('/api/v1/matches'),
      apiJSON<Profile>('/api/v1/profile')
    ]);
    if (matchesResult.status === 'fulfilled') {
      rawMatches = matchesResult.value.matches;
    } else {
      error = matchesResult.reason instanceof Error ? matchesResult.reason.message : 'Matches are unavailable.';
    }
    if (profileResult.status === 'fulfilled') {
      profile = profileResult.value;
    }
    loading = false;
  });

  function exportCsv() {
    const rows = [
      ['ID', 'Opponent', 'Score', 'Venue', 'Result', 'Last updated'],
      ...filtered.map((match) => [
        match.id,
        match.opponent,
        match.score,
        match.venue,
        match.result === 'W' ? 'Win' : 'Loss',
        formatDate(match.playedAt)
      ])
    ];
    const csv = rows
      .map((row) => row.map((cell) => `"${String(cell).replaceAll('"', '""')}"`).join(','))
      .join('\n');
    const url = URL.createObjectURL(new Blob([csv], { type: 'text/csv;charset=utf-8' }));
    const link = document.createElement('a');
    link.href = url;
    link.download = 'tennis-connect-matches.csv';
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 0);
  }
</script>

<svelte:head>
  <title>Overview · Tennis Connect</title>
</svelte:head>

<section class="page stack dashboard-page">
  {#if loading}
    <LoadingSkeleton variant="dashboard" />
  {:else}
    {#if error}
      <p class="alert-box" role="alert">{error}</p>
    {/if}

    <div class="dashboard-meta">
      <p class="sample-note" role="note">
        <span class="chip optic sample-chip">Sample</span>
        Scores, venues, results and court hours are illustrative. Opponents, dates, matches and play
        windows come from your account.
      </p>
      <a class="btn btn-primary btn-sm" href="/discover"><i class="bi bi-search" aria-hidden="true"></i> Find a match</a>
    </div>

    <section class="kpi-strip" aria-label="Key metrics">
      {#each kpis as kpi}
        <article class="kpi">
          <h2 class="kpi-label">{kpi.label}</h2>
          <p class="kpi-value">{kpi.value}{#if kpi.unit}<span class="kpi-unit">{kpi.unit}</span>{/if}</p>
          <p class="kpi-delta" class:up={kpi.direction === 'up'} class:down={kpi.direction === 'down'}>
            {#if kpi.direction === 'up'}
              <i class="bi bi-arrow-up-right" aria-hidden="true"></i>
            {:else if kpi.direction === 'down'}
              <i class="bi bi-arrow-down-right" aria-hidden="true"></i>
            {/if}
            <span>{kpi.delta}</span>
          </p>
        </article>
      {/each}
    </section>

    <div class="dash-grid">
      <section class="sheet-card chart-card" aria-labelledby="activity-title">
        <div class="card-head">
          <div>
            <p class="kicker">Activity</p>
            <h2 id="activity-title">Matches over time</h2>
            <p class="big-number">{monthly.reduce((total, month) => total + month.count, 0)}</p>
            <p class="muted mb-0">matches in the selected period</p>
          </div>
          <label class="period-select">
            <span class="visually-hidden">Chart period</span>
            <select bind:value={period} onchange={() => (activeIndex = -1)}>
              <option value="12">Yearly</option>
              <option value="6">Last 6 months</option>
            </select>
          </label>
        </div>

        <figure class="chart-figure">
          <svg
            class="area-chart"
            viewBox={`0 0 ${chart.width} ${chart.height}`}
            role="slider"
            tabindex="0"
            aria-labelledby="activity-title"
            aria-describedby="chart-summary"
            aria-valuemin="1"
            aria-valuemax={Math.max(monthly.length, 1)}
            aria-valuenow={activeIndex >= 0 ? activeIndex + 1 : 1}
            aria-valuetext={inspectedText}
            onkeydown={inspectChart}
            onfocus={() => {
              if (activeIndex < 0 && monthly.length) activeIndex = monthly.length - 1;
            }}
            onblur={() => (activeIndex = -1)}
          >
            <desc id="chart-summary">{summary}</desc>
            <defs>
              <linearGradient id="activity-fill" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" class="chart-stop-start" />
                <stop offset="100%" class="chart-stop-end" />
              </linearGradient>
              <linearGradient id="activity-line" x1="0" y1="0" x2="1" y2="0">
                <stop offset="0%" class="chart-line-stop-start" />
                <stop offset="100%" class="chart-line-stop-end" />
              </linearGradient>
            </defs>
            {#each chart.ticks as tick}
              <line
                class="grid-line"
                x1={chart.left}
                x2={chart.width - chart.right}
                y1={tick.y}
                y2={tick.y}
              />
              <text class="axis-label" x={chart.left - 6} y={tick.y + 3} text-anchor="end">{tick.label}</text>
            {/each}
            <path class="chart-area" d={chart.area} fill="url(#activity-fill)" />
            <path class="chart-line-halo" d={chart.line} />
            <path class="chart-line" d={chart.line} stroke="url(#activity-line)" />
            {#each chart.points as point, index}
              <circle class="chart-dot" class:active={activeIndex === index} cx={point.x} cy={point.y} r={activeIndex === index ? 4.5 : 3} />
            {/each}
            {#each chart.points as point, index}
              <circle
                class="chart-hit"
                cx={point.x}
                cy={point.y}
                r="13"
                role="presentation"
                aria-hidden="true"
                onpointerenter={() => (activeIndex = index)}
                onpointerleave={() => (activeIndex = -1)}
                onpointerdown={() => (activeIndex = index)}
              />
            {/each}
            {#if activePoint && activeMonth && tooltip}
              <g class="chart-tip" transform={`translate(${tooltip.x}, ${tooltip.y})`} aria-hidden="true">
                <rect
                  class="chart-tip-card"
                  x="-58"
                  y={tooltip.below ? 0 : -52}
                  width="116"
                  height="44"
                  rx="10"
                />
                <path
                  class="chart-tip-caret"
                  d={tooltip.below ? 'M-5,0 L0,-6 L5,0 Z' : 'M-5,-8 L0,-2 L5,-8 Z'}
                />
                <text class="chart-tip-month" x="0" y={tooltip.below ? 16 : -36} text-anchor="middle">{activeMonth.label}</text>
                <text class="chart-tip-value" x="0" y={tooltip.below ? 32 : -20} text-anchor="middle">
                  {activeMonth.count} {activeMonth.count === 1 ? 'match' : 'matches'}
                </text>
              </g>
            {/if}
            {#each monthly as month, index}
              <text
                class="axis-label"
                x={chart.points[index]?.x ?? 0}
                y={chart.height - 8}
                text-anchor="middle">{month.label}</text>
            {/each}
          </svg>
          <figcaption class="chart-summary muted">{summary}</figcaption>
        </figure>
      </section>

      <section class="sheet-card side-card" aria-labelledby="windows-title">
        <div class="card-head">
          <div>
            <p class="kicker">Availability</p>
            <h2 id="windows-title">Weekly play windows</h2>
          </div>
          <a href="/profile/play-times">Edit</a>
        </div>

        <p class="big-number">{windows.totalHours}<span class="unit">h</span></p>
        <p class="muted">available each week</p>

        {#if windows.windows.length}
          <ul class="progress-list">
            {#each windows.windows as slot}
              <li>
                <div class="progress-row">
                  <span>{slot.label}</span>
                  <span class="progress-value">{slot.hours}h</span>
                </div>
                <div class="segmented" role="img" aria-label={`${slot.label}: ${slot.hours} hours`}>
                  {#each segments as segment}
                    <span class="segment" class:on={segment < Math.round(slot.percent / 10)}></span>
                  {/each}
                </div>
              </li>
            {/each}
          </ul>
        {:else}
          <p class="muted mb-0">No play windows saved yet.</p>
        {/if}
      </section>
    </div>

    <section class="sheet-card table-card" aria-labelledby="recent-title">
      <div class="card-head table-head">
        <div>
          <p class="kicker">History</p>
          <h2 id="recent-title">Recent matches</h2>
          <p class="muted mb-0">Scores, venues and results are sample data.</p>
        </div>
        <div class="table-controls">
          <label class="control-search">
            <span class="visually-hidden">Search recent matches</span>
            <i class="bi bi-search" aria-hidden="true"></i>
            <input type="search" placeholder="Search" bind:value={query} />
          </label>
          <label>
            <span class="visually-hidden">Filter by result</span>
            <select bind:value={resultFilter}>
              <option value="all">All results</option>
              <option value="W">Wins</option>
              <option value="L">Losses</option>
            </select>
          </label>
          <button class="btn btn-outline-secondary btn-sm" type="button" onclick={exportCsv}>
            <i class="bi bi-download" aria-hidden="true"></i> Export
          </button>
        </div>
      </div>

      <div class="data-table-wrap" role="region" aria-labelledby="recent-title">
        <table class="data-table dashboard-table">
          <caption class="visually-hidden">Recent matches. Scores, venues and results are sample data.</caption>
          <thead>
            <tr>
              <th scope="col">ID</th>
              <th scope="col">Opponent</th>
              <th scope="col">Score</th>
              <th scope="col">Venue</th>
              <th scope="col">Result</th>
              <th scope="col">Last updated</th>
              <th scope="col"><span class="visually-hidden">Actions</span></th>
            </tr>
          </thead>
          <tbody>
            {#each filtered as match (match.matchId)}
              <tr>
                <td data-label="ID"><span class="mono">{match.id}</span></td>
                <td data-label="Opponent">{match.opponent}</td>
                <td data-label="Score">{match.score}</td>
                <td data-label="Venue">{match.venue}</td>
                <td data-label="Result">
                  <span class="chip" class:success={match.result === 'W'} class:danger={match.result === 'L'}>
                    <i class="bi" class:bi-trophy={match.result === 'W'} class:bi-dash-circle={match.result === 'L'} aria-hidden="true"></i>
                    {match.result === 'W' ? 'Win' : 'Loss'}
                  </span>
                </td>
                <td data-label="Updated">{formatDate(match.playedAt)}</td>
                <td data-label="Actions">
                  <a class="row-action" href={`/matches/${match.matchId}`} aria-label={`Actions for ${match.opponent}`}>
                    <i class="bi bi-three-dots" aria-hidden="true"></i>
                  </a>
                </td>
              </tr>
            {:else}
              <tr>
                <td colspan="7" class="empty-row">
                  No matches yet. <a href="/discover">Find a player</a> to start your history.
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </section>
  {/if}
</section>

<style>
  .dashboard-meta {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    flex-wrap: wrap;
  }

  .sample-note {
    display: flex;
    align-items: flex-start;
    gap: 0.5rem;
    margin: 0;
    color: var(--muted);
    font-size: 0.82rem;
  }

  .sample-chip {
    flex: 0 0 auto;
    font-size: 0.68rem;
    text-transform: uppercase;
    letter-spacing: 0.06em;
  }

  .dash-grid {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: var(--space-stack);
    align-items: stretch;
  }

  .chart-card,
  .side-card,
  .table-card {
    min-width: 0;
  }

  .card-head {
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 0.75rem;
    margin-bottom: 0.75rem;
  }

  .card-head h2 {
    margin: 0 0 0.2rem;
    font-size: 1rem;
  }

  .big-number {
    margin: 0.3rem 0 0;
    font-size: clamp(1.8rem, 3.4vw, 2.4rem);
    font-weight: 800;
    letter-spacing: -0.04em;
    line-height: 1;
  }

  .unit {
    margin-left: 0.15rem;
    color: var(--muted);
    font-size: 0.55em;
  }

  .period-select {
    flex: 0 0 auto;
  }

  .period-select select {
    width: auto;
    min-height: 2.2rem;
    padding: 0.25rem 0.6rem;
    font-size: 0.8rem;
  }

  .chart-figure {
    margin: 0;
  }

  .area-chart {
    display: block;
    width: 100%;
    height: auto;
    overflow: visible;
  }

  .chart-stop-start {
    stop-color: var(--chart-fill-start);
    stop-opacity: 0.34;
  }

  .chart-stop-end {
    stop-color: var(--chart-fill-start);
    stop-opacity: 0;
  }

  .chart-line-stop-start {
    stop-color: var(--chart-line-start);
  }

  .chart-line-stop-end {
    stop-color: var(--chart-line-end);
  }

  .chart-line-halo {
    fill: none;
    stroke: var(--chart-line-halo);
    stroke-width: 5.5;
    stroke-linecap: round;
    stroke-linejoin: round;
  }

  .chart-line {
    fill: none;
    stroke-width: 2.5;
    stroke-linecap: round;
    stroke-linejoin: round;
    filter: drop-shadow(0 0 6px var(--chart-glow));
  }

  .chart-dot {
    fill: var(--surface);
    stroke: var(--chart-line);
    stroke-width: 2;
    pointer-events: none;
    transition: r 140ms ease, fill 140ms ease, stroke 140ms ease;
  }

  .chart-dot.active {
    fill: var(--ball);
    stroke: var(--court-deep);
    stroke-width: 2.5;
    filter: drop-shadow(0 0 6px var(--ball-glow));
  }

  .area-chart:focus-visible {
    outline: 2px solid var(--sky);
    outline-offset: 4px;
    border-radius: 10px;
  }

  .chart-hit {
    fill: transparent;
    cursor: pointer;
  }

  .chart-tip {
    pointer-events: none;
    filter: var(--tooltip-shadow);
  }

  .chart-tip-card {
    fill: var(--surface);
    stroke: var(--line);
    stroke-width: 1;
  }

  .chart-tip-caret {
    fill: var(--surface);
    stroke: var(--line);
    stroke-width: 1;
    stroke-linejoin: round;
  }

  .chart-tip-month {
    fill: var(--muted);
    font-family: var(--font-mono);
    font-size: 9px;
    font-weight: 700;
    letter-spacing: 0.06em;
    text-transform: uppercase;
  }

  .chart-tip-value {
    fill: var(--ink);
    font-size: 12px;
    font-weight: 700;
  }

  .grid-line {
    stroke: var(--chart-grid);
    stroke-width: 1;
  }

  .axis-label {
    fill: var(--muted);
    font-family: var(--font-mono);
    font-size: 10px;
  }

  .chart-summary {
    margin: 0.4rem 0 0;
    font-size: 0.78rem;
  }

  .table-head {
    align-items: flex-start;
  }

  .table-controls {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.5rem;
  }

  @media (min-width: 768px) {
    .dashboard-table {
      min-width: 46rem;
    }
  }

  @media (min-width: 992px) {
    .dash-grid {
      grid-template-columns: minmax(0, 1.6fr) minmax(0, 1fr);
      gap: 1rem;
    }
  }

  @media (max-width: 767px) {
    .table-head {
      flex-direction: column;
    }

    .table-controls {
      width: 100%;
    }

    .table-controls .control-search {
      flex: 1 1 10rem;
    }

    .dashboard-meta .btn {
      width: 100%;
    }

    .dashboard-table td[data-label='ID'] {
      display: none;
    }

    .dashboard-table td[data-label='Opponent'] {
      order: 1;
      flex: 1 1 6rem;
      font-weight: 700;
    }

    .dashboard-table td[data-label='Result'] {
      order: 2;
      margin-left: auto;
    }

    .dashboard-table tbody tr::after {
      content: '';
      order: 3;
      flex-basis: 100%;
      height: 0;
    }

    .dashboard-table td[data-label='Score'] {
      order: 4;
    }

    .dashboard-table td[data-label='Venue'] {
      order: 5;
    }

    .dashboard-table td[data-label='Updated'] {
      order: 6;
    }

    .dashboard-table td[data-label='Score'],
    .dashboard-table td[data-label='Venue'],
    .dashboard-table td[data-label='Updated'] {
      color: var(--muted);
      font-size: 0.8rem;
      line-height: 1.3;
    }

    .dashboard-table td[data-label='Score']::after,
    .dashboard-table td[data-label='Venue']::after {
      content: '·';
      margin-left: 0.35rem;
    }

    .dashboard-table .chip {
      min-height: 1.5rem;
      padding: 0.1rem 0.5rem;
      font-size: 0.68rem;
    }
  }
</style>
