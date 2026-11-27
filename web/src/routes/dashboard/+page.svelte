<script lang="ts">
  import { onMount } from 'svelte';
  import { apiJSON, formatDate, type Match, type Profile } from '#lib/api';
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

  $: matches = buildDashboardMatches(rawMatches);
  $: kpis = buildKpis(matches);
  $: monthly = monthlyActivity(matches).slice(period === '6' ? -6 : 0);
  $: chart = chartGeometry(monthly.map((month) => month.count));
  $: summary = activitySummary(monthly);
  $: windows = playWindows(profile?.availability ?? []);
  $: filtered = filterMatches(matches, query, resultFilter);

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
  <p class="visually-hidden" role="status">{loading ? 'Loading dashboard' : ''}</p>

  {#if loading}
    <p role="status">Loading your dashboard…</p>
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
            <select bind:value={period}>
              <option value="12">Yearly</option>
              <option value="6">Last 6 months</option>
            </select>
          </label>
        </div>

        <figure class="chart-figure">
          <svg
            class="area-chart"
            viewBox={`0 0 ${chart.width} ${chart.height}`}
            role="img"
            aria-labelledby="chart-summary"
          >
            <desc id="chart-summary">{summary}</desc>
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
            <path class="chart-area" d={chart.area} />
            <path class="chart-line" d={chart.line} />
            {#each chart.points as point}
              <circle class="chart-dot" cx={point.x} cy={point.y} r="3" />
            {/each}
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
                <td><span class="mono">{match.id}</span></td>
                <td>{match.opponent}</td>
                <td>{match.score}</td>
                <td>{match.venue}</td>
                <td>
                  <span class="chip" class:success={match.result === 'W'} class:danger={match.result === 'L'}>
                    <i class="bi" class:bi-trophy={match.result === 'W'} class:bi-dash-circle={match.result === 'L'} aria-hidden="true"></i>
                    {match.result === 'W' ? 'Win' : 'Loss'}
                  </span>
                </td>
                <td>{formatDate(match.playedAt)}</td>
                <td>
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
    grid-template-columns: minmax(0, 1.6fr) minmax(0, 1fr);
    gap: 1rem;
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

  .chart-area {
    fill: var(--chart-fill);
  }

  .chart-line {
    fill: none;
    stroke: var(--chart-line);
    stroke-width: 2;
  }

  .chart-dot {
    fill: var(--surface);
    stroke: var(--chart-line);
    stroke-width: 2;
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

  .dashboard-table {
    min-width: 46rem;
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

  @media (max-width: 900px) {
    .dash-grid {
      grid-template-columns: minmax(0, 1fr);
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
  }
</style>
