document.addEventListener('DOMContentLoaded', () => {
  const token = document.querySelector('meta[name="auth-token"]').content;
  const appSelector = document.getElementById('appSelector');
  const timeSelector = document.getElementById('timeSelector');
  const timelineHeader = document.getElementById('timelineHeader');

  // Table Elements
  const serverTable = document.getElementById('serverTable');
  const tableBody = document.getElementById('tableBody');
  const searchInput = document.getElementById('searchTable');
  const btnPrev = document.getElementById('btnPrev');
  const btnNext = document.getElementById('btnNext');
  const pageInfo = document.getElementById('pageInfo');
  setupTableSort(serverTable);

  // Modal Logic
  const serverModal = new bootstrap.Modal(document.getElementById('serverModal'));
  const infoModal = new bootstrap.Modal(document.getElementById('infoModal'));
  const deleteModal = new bootstrap.Modal(document.getElementById('deleteModal'));
  const jsonContent = document.getElementById('jsonContent');
  const modalTitle = document.getElementById('modalTitle');
  const modalBody = document.getElementById('modalBody');

  // Delete logic state
  let targetToDelete = null;
  const deleteTargetText = document.getElementById('deleteTargetText');
  const btnConfirmDelete = document.getElementById('btnConfirmDelete');

  // Data State
  let rawData = [];
  let filteredData = []; // Data currently displayed (after app/time/search filters)
  let isoMap = {};
  let nameToIso = {};
  let currentCountry = null;
  let pieFilters = {
    country: null,
    os: null,
    version: null,
    map: null,
  };

  // Sort & Pagination State
  let sortKey = 'count';
  let sortDir = 'desc'; // 'asc' | 'desc'
  let currentPage = 1;
  const itemsPerPage = 15;

  const initChart = (id) => {
    const el = document.getElementById(id);
    return el ? echarts.init(el) : null;
  };

  const charts = {
    map: initChart('mapChart'),
    line: initChart('lineChart'),
    country: initChart('pieCountry'),
    os: initChart('pieOS'),
    version: initChart('pieVersion'),
    topServers: initChart('barTopServers'),
    mapName: initChart('pieMap'),
  };
  bindPieFilter(charts.country, 'country');
  bindPieFilter(charts.os, 'os');
  bindPieFilter(charts.version, 'version');
  bindPieFilter(charts.mapName, 'map');

  Object.values(charts).forEach(c => c && c.setOption({
    backgroundColor: 'transparent'
  }));

  window.addEventListener('resize', () => {
    Object.values(charts).forEach(c => c && c.resize());
  });

  // Handle actual delete click
  btnConfirmDelete.addEventListener('click', () => {
    if (targetToDelete) {
      executeDelete(targetToDelete);
    }
  });

  if (charts.map) {
    charts.map.on('click', function (params) {
      const countryName = params.name;
      const isoCode = nameToIso[countryName];
      if (!isoCode) return;
      if (currentCountry === isoCode) {
        currentCountry = null;
        charts.map.dispatchAction({
          type: 'unselect',
          name: countryName
        });
      } else {
        currentCountry = isoCode;
      }
      updateDashboard();
    });
  }

  const mapPromise = fetch('https://raw.githubusercontent.com/apache/echarts-examples/master/public/data/asset/geo/world.json').then(r => r.json());
  const statsPromise = fetch('/api/stats', {
    headers: {
      'Authorization': `Bearer ${token}`
    }
  }).then(r => r.json());
  const isoPromise = fetch('/data/iso3166.min.json').then(r => r.json());

  Promise.all([mapPromise, statsPromise, isoPromise])
    .then(([mapGeoJson, statsData, isoData]) => {
      echarts.registerMap('world', mapGeoJson);
      rawData = statsData || [];
      isoMap = isoData || {};
      Object.entries(isoMap).forEach(([code, name]) => nameToIso[name] = code);

      populateSelector(rawData);
      updateDashboard();
    })
    .catch(console.error);

  // --- EVENTS ---
  appSelector.addEventListener('change', () => updateDashboard());
  timeSelector.addEventListener('change', () => updateDashboard());

  // Table Search
  searchInput.addEventListener('input', () => {
    currentPage = 1;
    renderTable();
  });

  // Pagination
  btnPrev.addEventListener('click', () => {
    if (currentPage > 1) {
      currentPage--;
      renderTable();
    }
  });
  btnNext.addEventListener('click', () => {
    const maxPage = Math.ceil(filteredData.length / itemsPerPage);
    if (currentPage < maxPage) {
      currentPage++;
      renderTable();
    }
  });

  // --- LOGIC ---

  function populateSelector(data) {
    const apps = [...new Set(data.map(d => d.application))];
    apps.sort();
    apps.forEach(app => {
      const opt = document.createElement('option');
      opt.value = app;
      opt.innerText = app;
      appSelector.appendChild(opt);
    });
  }

  function updateDashboard() {
    const filterApp = appSelector.value;
    const filterTime = timeSelector.value;
    const now = new Date();
    let cutoff = new Date(0);

    if (filterTime === '24h') {
      cutoff = new Date(now.getTime() - (24 * 60 * 60 * 1000));
      timelineHeader.innerText = "Activity Timeline (Last 24 Hours)";
    } else if (filterTime === '7d') {
      cutoff = new Date(now.getTime() - (7 * 24 * 60 * 60 * 1000));
      timelineHeader.innerText = "Activity Timeline (Last 7 Days)";
    } else if (filterTime === '30d') {
      cutoff = new Date(now.getTime() - (30 * 24 * 60 * 60 * 1000));
      timelineHeader.innerText = "Activity Timeline (Last 30 Days)";
    } else {
      timelineHeader.innerText = "Activity Timeline (All Time)";
    }

    // Global Filter for Charts
    const data = rawData.filter(d => {
      const matchApp = (filterApp === 'all' || d.application === filterApp);
      const matchCountry =
        (currentCountry === null || d.country_code === currentCountry) &&
        (pieFilters.country === null || d.country_code === pieFilters.country);

      const matchOS = pieFilters.os === null || d.server_os === pieFilters.os;
      const matchVer = pieFilters.version === null || d.version === pieFilters.version;
      const matchMap = pieFilters.map === null || d.map_name === pieFilters.map;

      const seenTime = new Date(d.last_seen);
      const matchTime = seenTime >= cutoff;

      return matchApp && matchCountry && matchOS && matchVer && matchMap && matchTime;
    });

    calculateStats(data);
    renderCharts(data);

    // Reset Table
    currentPage = 1;
    // For table, we use the same filtered data as charts initially
    filteredData = [...data];
    sortKey = 'count';
    sortDir = 'desc';

    renderTable();
  }

  // --- TABLE LOGIC ---
  function renderTable() {
    const query = searchInput.value.toLowerCase();
    let displayData = filteredData;

    if (query) {
      displayData = filteredData.filter(d =>
        (d.server_name && d.server_name.toLowerCase().includes(query)) ||
        (d.ip && d.ip.includes(query))
      );
    }

    const toTs = (v) => {
      const t = Date.parse(v || '');
      return Number.isFinite(t) ? t : 0;
    };
    const toNum = (v) => Number.isFinite(Number(v)) ? Number(v) : 0;
    const toStr = (v) => (v ?? '').toString().toLowerCase();

    const val = (d, k) => {
      switch (k) {
        case 'address':
          return `${d.ip || ''}:${d.port || ''}`;
        case 'players':
          return toNum(d.players);
        case 'count':
          return toNum(d.count);
        case 'first_seen':
          return toTs(d.first_seen);
        case 'last_seen':
          return toTs(d.last_seen);
        default:
          return toStr(d[k]);
      }
    };

    if (sortKey) {
      const dir = (sortDir === 'asc') ? 1 : -1;
      displayData = [...displayData].sort((a, b) => {
        const va = val(a, sortKey);
        const vb = val(b, sortKey);
        if (typeof va === 'number' && typeof vb === 'number') return (va - vb) * dir;
        if (va < vb) return -1 * dir;
        if (va > vb) return 1 * dir;
        return 0;
      });
    }

    // Pagination
    const totalItems = displayData.length;
    const start = (currentPage - 1) * itemsPerPage;
    const end = start + itemsPerPage;
    const pageData = displayData.slice(start, end);


    // Render HTML
    tableBody.innerHTML = '';
    pageData.forEach((d, idx) => {
      const row = document.createElement('tr');

      // Format Last Seen
      const fmt = (iso) => {
        const dt = new Date(iso);
        if (Number.isNaN(dt.getTime())) return '-';
        return dt.toLocaleDateString() + ' ' + dt.toLocaleTimeString([], {
          hour: '2-digit',
          minute: '2-digit'
        });
      };

      const firstSeenStr = fmt(d.first_seen);
      const lastSeenStr = fmt(d.last_seen);

      // Flag
      const flag = d.country_code ? d.country_code : '🏳️';

      row.innerHTML = `
        <td>${start + idx + 1}</td>
        <td>
          <div class="text-truncate" style="max-width: 250px;" title="${d.server_name || 'N/A'}">
            ${d.server_name || '<span class="text-muted">Unknown</span>'}
          </div>
        </td>
        <td>
          <span class="badge bg-dark border border-secondary text-light font-monospace">${d.ip}:${d.port}</span> <small>${flag}</small>
        </td>
        <td>
          ${d.application}
          <span class="badge bg-dark border border-secondary text-light font-monospace">${d.version}</span>
        </td>
        <td>${d.map_name || '-'}</td>
        <td><span class="text-success">${d.players}</span> / <span class="text-muted">${d.max_players}</span></td>
        <td><strong>${d.count}</strong></td>
        <td class="small text-muted">${firstSeenStr}</td>
        <td class="small text-muted">${lastSeenStr}</td>
        <td>
          <div class="btn-group btn-group-sm" role="group">
            <button class="btn btn-outline-secondary" title="Ping A2S"
              onclick="pingServer(this,'${d.ip}', ${d.port}, '${escapeHtml(d.server_name || '')}')">
              Ping
            </button>
            <button class="btn btn-outline-info" title="View JSON"
              onclick="showInfo('${d.application}', '${d.ip}', ${d.port})">
              Info
            </button>
            <button class="btn btn-outline-danger" title="Delete Node"
              onclick="askDelete('${d.application}', '${d.ip}', ${d.port})">
              Del
            </button>
          </div>
        </td>
      `;
      tableBody.appendChild(row);
    });

    // Update Controls
    pageInfo.innerText = `Showing ${Math.min(start + 1, totalItems)}-${Math.min(end, totalItems)} of ${totalItems}`;
    btnPrev.disabled = currentPage === 1;
    btnNext.disabled = end >= totalItems;
  }

  // --- PING MODAL LOGIC (Global Scope) ---
  window.pingServer = function (btn, ip, port, name) {
    // Reset button state for this click
    if (btn) {
      btn.disabled = true;
      btn.classList.remove('btn-ping-ok', 'btn-ping-warn');
      btn.textContent = '...';
    }

    modalTitle.innerText = "Querying Server...";
    modalBody.innerHTML = '<div class="text-center py-4"><div class="spinner-border text-success" role="status"></div><div class="mt-2 text-muted">Contacting Server...</div></div>';
    serverModal.show();

    fetch(`/api/a2s?ip=${encodeURIComponent(ip)}&port=${encodeURIComponent(port)}`, {
        headers: {
          'Authorization': `Bearer ${token}`
        }
      })
      .then(r => r.json())
      .then(info => {
        if (info.error) {
          if (btn) {
            btn.disabled = false;
            btn.classList.add('btn-ping-warn');
            btn.textContent = 'Offline';
          }
          modalTitle.innerText = "Server Offline";
          modalBody.innerHTML = `<div class="alert alert-danger">Failed to query server: ${info.error}</div>`;
          return;
        }

        modalTitle.innerText = info.name || name || "Server Details";

        const pingMsNum = Number(info.ping || 0) / 1e6;
        const pingMs = pingMsNum.toFixed(1);

        if (btn) {
          btn.disabled = false;
          btn.classList.add('btn-ping-ok');
          btn.textContent = `${pingMs} ms`;
        }

        const rows = [
          ['Query Address', `${ip}:${port}`],
          ['Game Address', `${ip}:${info.port}`],
          ['Name', info.name || '-'],
          ['Description', info.game || '-'],
          ['Game', info.folder || '-'],
          ['Map', info.map || '-'],
          ['Players', `${info.players} / ${info.max_players} (Bots: ${info.bots || 0})`],
          ['Version', info.version || '-'],
          ['OS', info.environment || '-'],
          ['Password', info.public ? '<span class="text-danger">Protected</span>' : '<span class="text-success">No Password</span>'],
          ['Ping', `${pingMs} ms`],
        ];

        let html = '<div class="px-2">';
        rows.forEach(([label, val]) => {
          html += `
          <div class="server-detail-row">
            <span class="detail-label">${label}</span>
            <span class="detail-value">${val}</span>
          </div>`;
        });
        html += '</div>';
        modalBody.innerHTML = html;
      })
      .catch(err => {
        if (btn) {
          btn.disabled = false;
          btn.classList.add('btn-ping-warn');
          btn.textContent = 'Error';
        }
        modalTitle.innerText = "Error";
        modalBody.innerHTML = `<div class="alert alert-danger">Network Error: ${err.message}</div>`;
      });
  };

  // --- INFO MODAL LOGIC ---
  window.showInfo = function (app, ip, port) {
    jsonContent.innerText = "Loading...";
    infoModal.show();

    const params = new URLSearchParams({
      app,
      ip,
      port
    });
    fetch(`/api/node?${params.toString()}`, {
        headers: {
          'Authorization': `Bearer ${token}`
        }
      })
      .then(r => {
        if (!r.ok) throw new Error("Not found");
        return r.json();
      })
      .then(data => {
        jsonContent.innerText = JSON.stringify(data, null, 2);
      })
      .catch(err => {
        jsonContent.innerText = "Error loading data: " + err.message;
      });
  };

  // --- DELETE MODAL LOGIC ---
  window.askDelete = function (app, ip, port) {
    targetToDelete = {
      app,
      ip,
      port
    };
    deleteTargetText.innerText = `${app} @ ${ip}:${port}`;
    deleteModal.show();
  };

  // Helpers
  function escapeHtml(text) {
    if (!text) return text;
    return text
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;")
      .replace(/'/g, "&#039;");
  }

  function calculateStats(data) {
    // total
    animateValue("valTotal", data.length);

    // unique online IP:Port
    const online = data.filter(d => d.game_version);
    const uniqueServers = new Set(online.map(d => `${d.ip}:${d.port}`));
    animateValue("valUnique", uniqueServers.size);

    // unique IP (host)
    const uniqueHosts = new Set(data.map(d => d.ip));
    animateValue("valHosts", uniqueHosts.size);

    // players
    let totalPlayers = 0;
    const processed = new Set();
    data.forEach(d => {
      const key = `${d.ip}:${d.port}`;
      if (!processed.has(key)) {
        totalPlayers += (d.players || 0);
        processed.add(key);
      }
    });
    animateValue("valPlayers", totalPlayers);
  }

  function renderCharts(data) {
    if (charts.map) renderMap(data);
    if (charts.line) renderTimeline(data);
    if (charts.topServers) renderTopServers(data);
    const pieConfig = (name, data) => ({
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'item',
        formatter: '{b}: {c} ({d}%)'
      },
      legend: {
        show: false
      },
      grid: {
        top: 0,
        bottom: 0,
        left: 0,
        right: 0
      },
      color: ['#00ab44', '#00a8e8', '#e8a800', '#8931ef', '#f44336', '#00bcd4'],
      series: [{
        name: name,
        type: 'pie',
        radius: ['45%', '70%'],
        center: ['50%', '50%'],
        itemStyle: {
          borderRadius: 3,
          borderColor: '#2c2e33',
          borderWidth: 2
        },
        label: {
          show: true,
          position: 'inside',
          formatter: '{c}',
          color: '#fff',
          fontSize: 10
        },
        labelLine: {
          show: false
        },
        data: data
      }]
    });
    if (charts.country) charts.country.setOption(pieConfig('Countries', getTopN(data, 'country_code', 10)));
    if (charts.os) charts.os.setOption(pieConfig('OS', getGrouped(data, 'server_os')));
    if (charts.version) charts.version.setOption(pieConfig('App Version', getGrouped(data, 'version')));
    if (charts.mapName) charts.mapName.setOption(pieConfig('Maps', getGrouped(data, 'map_name')));
  }

  function renderMap(data) {
    const counts = {};
    data.forEach(d => {
      const cc = d.country_code ? d.country_code.toUpperCase() : "UNKNOWN";
      const fullName = isoMap[cc] || cc;
      counts[fullName] = (counts[fullName] || 0) + 1;
    });
    const mapData = Object.keys(counts).map(k => ({
      name: k,
      value: counts[k],
      selected: (nameToIso[k] === currentCountry)
    }));
    charts.map.setOption({
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'item',
        formatter: function (params) {
          const val = params.value || 0;
          return `${params.name}: ${val} servers`;
        }
      },
      visualMap: {
        min: 0,
        max: Math.max(...Object.values(counts)) || 1,
        left: '20',
        bottom: '20',
        text: ['High', 'Low'],
        realtime: false,
        calculable: true,
        inRange: {
          color: ['#2c2e33', '#1e3c72', '#00ab44']
        },
        textStyle: {
          color: '#9fa5b0'
        }
      },
      geo: {
        map: 'world',
        roam: true,
        zoom: 1.2,
        selectedMode: 'single',
        label: {
          emphasis: {
            show: false
          }
        },
        itemStyle: {
          normal: {
            areaColor: '#373b41',
            borderColor: '#202226'
          },
          emphasis: {
            areaColor: '#00a8e8'
          }
        },
        select: {
          itemStyle: {
            areaColor: '#00f260',
            borderColor: '#fff'
          },
          label: {
            show: false
          }
        }
      },
      series: [{
        type: 'map',
        geoIndex: 0,
        data: mapData
      }]
    });
  }

  function renderTimeline(data) {
    const filterTime = timeSelector.value;
    const sorted = [...data].sort((a, b) => new Date(a.last_seen) - new Date(b.last_seen));
    const timeBuckets = {};
    sorted.forEach(d => {
      const date = new Date(d.last_seen);
      if (filterTime === '30d' || filterTime === 'all' || filterTime === '7d') {
        date.setHours(0, 0, 0, 0);
      } else {
        date.setMinutes(0, 0, 0);
      }
      const key = date.toISOString();
      timeBuckets[key] = (timeBuckets[key] || 0) + 1;
    });
    const keys = Object.keys(timeBuckets).sort();
    const values = keys.map(k => timeBuckets[k]);
    const labels = keys.map(k => {
      const d = new Date(k);
      if (filterTime === '24h') return d.toLocaleTimeString([], {
        hour: '2-digit',
        minute: '2-digit'
      });
      return d.toLocaleDateString([], {
        month: 'short',
        day: 'numeric'
      });
    });
    charts.line.setOption({
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'axis'
      },
      grid: {
        left: '30',
        right: '30',
        bottom: '20',
        top: '10',
        containLabel: true
      },
      xAxis: {
        type: 'category',
        data: labels,
        boundaryGap: false
      },
      yAxis: {
        type: 'value',
        splitLine: {
          lineStyle: {
            color: '#373b41'
          }
        }
      },
      series: [{
        name: 'Active Servers',
        type: 'line',
        smooth: true,
        showSymbol: false,
        data: values,
        lineStyle: {
          color: '#00ab44',
          width: 2
        },
        areaStyle: {
          color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [{
            offset: 0,
            color: 'rgba(0, 171, 68, 0.5)'
          }, {
            offset: 1,
            color: 'transparent'
          }])
        }
      }]
    });
  }

  function renderTopServers(data) {
    const top = [...data].sort((a, b) => b.count - a.count).slice(0, 20);
    top.reverse();
    const names = top.map(d => d.server_name || d.ip);
    const values = top.map(d => d.count);
    charts.topServers.setOption({
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'axis',
        axisPointer: {
          type: 'shadow'
        }
      },
      grid: {
        left: '10',
        right: '30',
        bottom: '10',
        top: '10',
        containLabel: true
      },
      xAxis: {
        type: 'value',
        splitLine: {
          lineStyle: {
            color: '#373b41'
          }
        }
      },
      yAxis: {
        type: 'category',
        data: names,
        axisLabel: {
          width: 140,
          overflow: 'truncate',
          color: '#9fa5b0',
          fontSize: 10
        }
      },
      series: [{
        name: 'Requests',
        type: 'bar',
        data: values,
        itemStyle: {
          color: '#00a8e8',
          borderRadius: [0, 3, 3, 0]
        },
        barWidth: '60%'
      }]
    });
  }

  function getGrouped(data, field) {
    const counts = {};
    data.forEach(d => {
      const val = d[field] || 'Unknown';
      counts[val] = (counts[val] || 0) + 1;
    });
    return Object.keys(counts).map(k => ({
      name: k,
      value: counts[k]
    }));
  }

  function getTopN(data, field, n) {
    const counts = {};
    data.forEach(d => {
      const val = d[field] || 'Unknown';
      counts[val] = (counts[val] || 0) + 1;
    });
    return Object.entries(counts).sort((a, b) => b[1] - a[1]).slice(0, n).map(([name, value]) => ({
      name,
      value
    }));
  }

  function bindPieFilter(chart, key) {
    if (!chart) return;
    chart.on('click', params => {
      const v = params.name;
      pieFilters[key] = (pieFilters[key] === v) ? null : v;
      updateDashboard();
    });
  }

  function setupTableSort(table) {
    if (!table) return;
    table.querySelectorAll('thead th[data-sort]').forEach(th => {
      th.style.cursor = 'pointer';
      th.addEventListener('click', () => {
        const key = th.dataset.sort;
        if (sortKey === key) sortDir = (sortDir === 'asc' ? 'desc' : 'asc');
        else {
          sortKey = key;
          sortDir = 'asc';
        }
        currentPage = 1;
        renderTable();
      });
    });
  }

  function executeDelete(target) {
    const params = new URLSearchParams(target);

    btnConfirmDelete.disabled = true;
    btnConfirmDelete.innerText = "Deleting...";

    fetch(`/api/node?${params.toString()}`, {
        method: 'DELETE',
        headers: {
          'Authorization': `Bearer ${token}`
        }
      })
      .then(r => r.json())
      .then(res => {
        if (res.status === 'ok') {
          rawData = rawData.filter(d => !(d.application === target.app && d.ip === target.ip && d.port === target.port));
          updateDashboard();

          deleteModal.hide();
        } else {
          alert("Error: " + (res.message || "Unknown error"));
        }
      })
      .catch(err => {
        alert("Request failed: " + err.message);
      })
      .finally(() => {
        btnConfirmDelete.disabled = false;
        btnConfirmDelete.innerText = "Delete Server";
        targetToDelete = null;
      });
  }

  function animateValue(id, end) {
    const obj = document.getElementById(id);
    if (!obj) return;
    const start = parseInt(obj.innerHTML) || 0;
    if (start === end) return;
    const duration = 1000;
    const startTime = performance.now();

    function update(currentTime) {
      const elapsed = currentTime - startTime;
      const progress = Math.min(elapsed / duration, 1);
      const ease = 1 - Math.pow(1 - progress, 4);
      obj.innerHTML = Math.floor(start + (end - start) * ease);
      if (progress < 1) requestAnimationFrame(update);
      else obj.innerHTML = end;
    }
    requestAnimationFrame(update);
  }
});
