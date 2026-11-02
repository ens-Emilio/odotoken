import { motion, useScroll, useTransform } from 'motion/react'
import { useRef } from 'react'
import { Activity, ShieldCheck, Zap, BarChart3, Globe, Terminal, ArrowRight, Layers, Lock } from 'lucide-react'

function Hero() {
  return (
    <section className="relative flex min-h-[90vh] flex-col items-center justify-center overflow-hidden px-6 text-center">
      {/* Background Grid Animation */}
      <div className="pointer-events-none absolute inset-0 z-0 opacity-20">
        <div className="absolute inset-0 bg-[linear-gradient(to_right,#80808012_1px,transparent_1px),linear-gradient(to_bottom,#80808012_1px,transparent_1px)] bg-[size:24px_24px]" />
        <motion.div
          className="absolute left-1/2 top-1/2 h-[500px] w-[500px] -translate-x-1/2 -translate-y-1/2 rounded-full bg-[var(--accent)] blur-[120px]"
          animate={{ scale: [1, 1.2, 1], opacity: [0.3, 0.5, 0.3] }}
          transition={{ duration: 8, repeat: Infinity, ease: 'easeInOut' }}
        />
      </div>

      <motion.div
        initial={{ opacity: 0, y: 20 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.8, ease: [0.22, 1, 0.36, 1] }}
        className="relative z-10 max-w-4xl"
      >
        <motion.div
          className="mb-6 inline-flex items-center gap-2 rounded-full border border-[var(--border)] bg-[var(--bg-panel)] px-4 py-1.5 text-xs font-medium text-[var(--text-muted)]"
          initial={{ opacity: 0, scale: 0.9 }}
          animate={{ opacity: 1, scale: 1 }}
          transition={{ delay: 0.2 }}
        >
          <span className="relative flex h-2 w-2">
            <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-400 opacity-75" />
            <span className="relative inline-flex h-2 w-2 rounded-full bg-emerald-500" />
          </span>
          v0.8.1 Available Now
        </motion.div>

        <h1 className="mb-6 text-5xl font-bold tracking-tight text-[var(--text-strong)] sm:text-7xl md:text-8xl">
          <span className="block">AI Agent</span>
          <span className="bg-gradient-to-r from-emerald-400 via-teal-400 to-cyan-400 bg-clip-text text-transparent">
            Observability
          </span>
        </h1>

        <p className="mx-auto mb-10 max-w-2xl text-lg text-[var(--text-muted)] sm:text-xl">
          Intercepte requisições LLM, calcule custos reais em tempo real e imponha limites de orçamento — sem alterar seu workflow.
        </p>

        <motion.div
          className="flex flex-wrap items-center justify-center gap-4"
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.4 }}
        >
          <a href="#overview" className="group flex items-center gap-2 rounded-lg bg-emerald-500 px-6 py-3 text-sm font-semibold text-white shadow-lg shadow-emerald-500/20 transition-all hover:bg-emerald-400 hover:shadow-emerald-500/30">
            Ver Dashboard Demo
            <ArrowRight className="h-4 w-4 transition-transform group-hover:translate-x-1" />
          </a>
          <a href="https://github.com/ens-Emilio/odotoken" target="_blank" rel="noreferrer" className="flex items-center gap-2 rounded-lg border border-[var(--border)] bg-[var(--bg-panel)] px-6 py-3 text-sm font-semibold text-[var(--text-strong)] transition-colors hover:bg-[var(--bg-fill-secondary)]">
            <Terminal className="h-4 w-4" />
            GitHub
          </a>
        </motion.div>
      </motion.div>

      {/* Floating Architecture Diagram Hint */}
      <motion.div
        className="absolute bottom-12 left-1/2 -translate-x-1/2"
        animate={{ y: [0, 10, 0] }}
        transition={{ duration: 4, repeat: Infinity, ease: 'easeInOut' }}
      >
        <div className="flex items-center gap-3 rounded-full border border-[var(--border)] bg-[var(--glass-regular)] px-4 py-2 backdrop-blur-md">
          <div className="flex -space-x-2">
            {[1, 2, 3].map((i) => (
              <div key={i} className="h-6 w-6 rounded-full border-2 border-[var(--bg-elevated)] bg-[var(--bg-fill-secondary)]" />
            ))}
          </div>
          <span className="text-[10px] font-medium uppercase tracking-wider text-[var(--text-faint)]">CLI → Proxy → LLM</span>
        </div>
      </motion.div>
    </section>
  )
}

function FeatureCard({ icon: Icon, title, desc, delay }: { icon: React.ElementType; title: string; desc: string; delay: number }) {
  return (
    <motion.div
      initial={{ opacity: 0, y: 20 }}
      whileInView={{ opacity: 1, y: 0 }}
      viewport={{ once: true, margin: '-100px' }}
      transition={{ duration: 0.5, delay }}
      className="group relative overflow-hidden rounded-2xl border border-[var(--border)] bg-[var(--bg-panel)] p-8 transition-colors hover:border-emerald-500/30 hover:bg-[var(--bg-elevated)]"
    >
      <div className="absolute -right-6 -top-6 h-24 w-24 rounded-full bg-emerald-500/5 transition-transform group-hover:scale-150" />
      <div className="mb-4 inline-flex rounded-lg bg-emerald-500/10 p-3 text-emerald-500">
        <Icon className="h-6 w-6" />
      </div>
      <h3 className="mb-2 text-lg font-semibold text-[var(--text-strong)]">{title}</h3>
      <p className="text-sm leading-relaxed text-[var(--text-muted)]">{desc}</p>
    </motion.div>
  )
}

function FeaturesGrid() {
  const features = [
    { icon: Activity, title: 'Metering Transparente', desc: 'Captura tokens de JSON e SSE bodies automaticamente. Zero mudança no código do seu agente.' },
    { icon: BarChart3, title: 'Custos Reais', desc: 'Base de preços com 2.700+ modelos atualizada. Projeção linear e detecção de anomalias.' },
    { icon: ShieldCheck, title: 'Budget Guard', desc: 'Limites diários/mensais por projeto ou chave. Alerta ou bloqueia com HTTP 429.' },
    { icon: Globe, title: 'Dashboard Global', desc: 'Interface em 20 idiomas com suporte RTL. Temas claro/escuro e 21 paletas de cores.' },
    { icon: Zap, title: 'Throughput Analytics', desc: 'Métricas de TPS e TTFT por modelo. Latência detalhada e cache savings.' },
    { icon: Lock, title: 'Segurança First', desc: 'Bind local por padrão. Chaves nunca são armazenadas, apenas repassadas via header.' },
  ]

  return (
    <section className="px-6 py-24">
      <div className="mx-auto max-w-6xl">
        <motion.div
          className="mb-16 text-center"
          initial={{ opacity: 0 }}
          whileInView={{ opacity: 1 }}
          viewport={{ once: true }}
        >
          <h2 className="mb-4 text-3xl font-bold text-[var(--text-strong)] sm:text-4xl">Tudo que você precisa para governar IA</h2>
          <p className="text-[var(--text-muted)]">Infraestrutura completa para times que escalam agentes de IA em produção.</p>
        </motion.div>
        <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {features.map((f, i) => (
            <FeatureCard key={f.title} {...f} delay={i * 0.1} />
          ))}
        </div>
      </div>
    </section>
  )
}

function ArchitectureFlow() {
  return (
    <section className="overflow-hidden bg-[var(--bg-fill-secondary)] px-6 py-24">
      <div className="mx-auto max-w-5xl">
        <motion.div
          className="mb-12 text-center"
          initial={{ opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
        >
          <h2 className="mb-4 text-3xl font-bold text-[var(--text-strong)]">Como Funciona</h2>
          <p className="text-[var(--text-muted)]">Arquitetura de proxy leve com persistência SQLite e dashboard embutido.</p>
        </motion.div>

        <div className="relative flex flex-col items-center justify-between gap-8 sm:flex-row sm:gap-4">
          {/* Nodes */}
          {[
            { label: 'CLI Agents', sub: 'Cursor, Aider, etc', color: 'bg-blue-500' },
            { label: 'OdoToken Proxy', sub: ':8080 • Metering', color: 'bg-emerald-500' },
            { label: 'LLM Providers', sub: 'OpenAI, Anthropic...', color: 'bg-purple-500' },
          ].map((node, i) => (
            <motion.div
              key={node.label}
              className="relative z-10 flex w-full flex-col items-center gap-3 rounded-xl border border-[var(--border)] bg-[var(--bg-panel)] p-6 text-center shadow-sm sm:w-1/3"
              initial={{ opacity: 0, scale: 0.9 }}
              whileInView={{ opacity: 1, scale: 1 }}
              viewport={{ once: true }}
              transition={{ delay: i * 0.2 }}
            >
              <div className={`h-3 w-3 rounded-full ${node.color}`} />
              <div className="font-semibold text-[var(--text-strong)]">{node.label}</div>
              <div className="text-xs text-[var(--text-muted)]">{node.sub}</div>
            </motion.div>
          ))}

          {/* Arrows (Desktop only visual connector) */}
          <div className="absolute left-0 top-1/2 hidden h-0.5 w-full -translate-y-1/2 bg-gradient-to-r from-blue-500/20 via-emerald-500/50 to-purple-500/20 sm:block" />
        </div>

        <motion.div
          className="mt-12 grid gap-4 sm:grid-cols-3"
          initial={{ opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
          transition={{ delay: 0.6 }}
        >
          {[
            { icon: Layers, text: 'SQLite History + WAL' },
            { icon: BarChart3, text: 'Real-time Cost Calc' },
            { icon: ShieldCheck, text: 'Guard & Rate Limits' },
          ].map((item) => (
            <div key={item.text} className="flex items-center justify-center gap-2 rounded-lg border border-dashed border-[var(--border)] bg-[var(--bg-panel)]/50 py-3 text-xs font-medium text-[var(--text-muted)]">
              <item.icon className="h-3.5 w-3.5 text-emerald-500" />
              {item.text}
            </div>
          ))}
        </motion.div>
      </div>
    </section>
  )
}

export function ShowcaseView() {
  const containerRef = useRef<HTMLDivElement>(null)
  const { scrollYProgress } = useScroll({ target: containerRef })
  const scaleX = useTransform(scrollYProgress, [0, 1], [0, 1])

  return (
    <div ref={containerRef} className="relative pb-24">
      {/* Progress bar */}
      <motion.div className="fixed left-0 right-0 top-0 z-50 h-1 origin-left bg-emerald-500" style={{ scaleX }} />

      <Hero />
      <FeaturesGrid />
      <ArchitectureFlow />

      <section className="px-6 py-24 text-center">
        <motion.div
          className="mx-auto max-w-2xl rounded-3xl border border-[var(--border)] bg-[var(--bg-panel)] p-12 shadow-2xl shadow-emerald-500/5"
          initial={{ opacity: 0, y: 30 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
        >
          <h2 className="mb-6 text-3xl font-bold text-[var(--text-strong)]">Pronto para assumir o controle?</h2>
          <p className="mb-8 text-[var(--text-muted)]">Instale via binário pré-compilado ou build from source. Setup em menos de 2 minutos.</p>
          <div className="flex flex-col items-center gap-4 sm:flex-row sm:justify-center">
            <a href="https://github.com/ens-Emilio/odotoken/releases/latest" className="rounded-lg bg-emerald-500 px-8 py-3 text-sm font-semibold text-white transition-colors hover:bg-emerald-400">
              Download v0.8.1
            </a>
            <code className="rounded-lg border border-[var(--border)] bg-[var(--bg-fill-secondary)] px-6 py-3 text-xs text-[var(--text-muted)]">
              go build -o bin/odotoken ./cmd/odotoken
            </code>
          </div>
        </motion.div>
      </section>

      <footer className="border-t border-[var(--border)] px-6 py-12 text-center text-xs text-[var(--text-faint)]">
        <p>OdoToken © {new Date().getFullYear()} • GPL-3.0-or-later • Made for the AI Engineering Community</p>
      </footer>
    </div>
  )
}