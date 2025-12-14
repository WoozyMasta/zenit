document.addEventListener('DOMContentLoaded', () => {
  // --- PART 1: LOCALIZATION ---
  const langBtn = document.getElementById('langBtn');
  let currentLang = localStorage.getItem('lang') || (navigator.language.startsWith('ru') ? 'ru' : 'en');
  let translations = {};

  function setLanguage(lang) {
    currentLang = lang;
    localStorage.setItem('lang', lang);

    // Update Text
    document.querySelectorAll('[data-i18n]').forEach(el => {
      const key = el.getAttribute('data-i18n');
      if (translations[lang] && translations[lang][key]) {
        el.innerText = translations[lang][key];
      }
    });

    // Update Button text (showing the OTHER language option)
    if (translations[lang]) {
      langBtn.innerText = translations[lang]['lang_switch'];
    }
  }

  // Load Translations
  fetch('/data/locales.min.json')
    .then(r => r.json())
    .then(data => {
      translations = data;
      setLanguage(currentLang);
    });

  // Switch Handler
  langBtn.addEventListener('click', () => {
    const newLang = currentLang === 'en' ? 'ru' : 'en';
    setLanguage(newLang);
  });


  // --- PART 2: CANVAS ANIMATION (Network Nodes) ---
  const canvas = document.getElementById('bg-canvas');
  const ctx = canvas.getContext('2d');

  let width, height;
  let particles = [];

  // Config
  const particleCount = window.innerWidth < 600 ? 30 : 60;
  const connectionDistance = 150;
  const particleSpeed = 0.5;

  function resize() {
    width = canvas.width = window.innerWidth;
    height = canvas.height = window.innerHeight;
  }

  class Particle {
    constructor() {
      this.x = Math.random() * width;
      this.y = Math.random() * height;
      this.vx = (Math.random() - 0.5) * particleSpeed;
      this.vy = (Math.random() - 0.5) * particleSpeed;
      this.size = Math.random() * 2 + 1;
    }

    update() {
      this.x += this.vx;
      this.y += this.vy;

      // Bounce off edges
      if (this.x < 0 || this.x > width) this.vx *= -1;
      if (this.y < 0 || this.y > height) this.vy *= -1;
    }

    draw() {
      ctx.beginPath();
      ctx.arc(this.x, this.y, this.size, 0, Math.PI * 2);
      ctx.fillStyle = 'rgba(0, 242, 96, 0.5)'; // Accent color dots
      ctx.fill();
    }
  }

  function initParticles() {
    particles = [];
    for (let i = 0; i < particleCount; i++) {
      particles.push(new Particle());
    }
  }

  function animate() {
    ctx.clearRect(0, 0, width, height);

    // Update and draw particles
    for (let i = 0; i < particles.length; i++) {
      particles[i].update();
      particles[i].draw();

      // Draw connections
      for (let j = i; j < particles.length; j++) {
        const dx = particles[i].x - particles[j].x;
        const dy = particles[i].y - particles[j].y;
        const distance = Math.sqrt(dx * dx + dy * dy);

        if (distance < connectionDistance) {
          ctx.beginPath();
          ctx.strokeStyle = `rgba(0, 242, 96, ${0.15 - distance/connectionDistance * 0.15})`; // Fading line
          ctx.lineWidth = 1;
          ctx.moveTo(particles[i].x, particles[i].y);
          ctx.lineTo(particles[j].x, particles[j].y);
          ctx.stroke();
        }
      }
    }
    requestAnimationFrame(animate);
  }

  window.addEventListener('resize', () => {
    resize();
    initParticles();
  });

  resize();
  initParticles();
  animate();
});
