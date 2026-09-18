import Card from '../../components/ui/Card'
import Button from '../../components/ui/Button'
import { Badge } from '../../components/ui/Badge'
import './ExamServerPanel.css'

export default function ExamServerPanel({ info }) {
  if (!info) return null

  const copyUrl = () => navigator.clipboard?.writeText(info.access_url)

  return (
    <Card>
      <div className="es-server-panel">
        <div className="es-server-panel__status">
          <Badge tone="success">Server online</Badge>
          <span className="es-server-panel__iface">via {info.network_interface}</span>
        </div>

        <div className="es-server-panel__grid">
          <div>
            <div className="es-server-panel__row">
              <span className="es-server-panel__label">Server IP</span>
              <span className="es-server-panel__value">{info.ip_address}</span>
            </div>
            <div className="es-server-panel__row">
              <span className="es-server-panel__label">Port</span>
              <span className="es-server-panel__value">{info.port}</span>
            </div>
            <div className="es-server-panel__row">
              <span className="es-server-panel__label">Exam access URL</span>
              <span className="es-server-panel__value es-server-panel__url">{info.access_url}</span>
            </div>
            <div style={{ display: 'flex', gap: 8, marginTop: 12 }}>
              <Button size="sm" variant="secondary" onClick={copyUrl}>Copy exam URL</Button>
            </div>
            <p className="es-server-panel__hint">
              Students on the same Wi-Fi/LAN can enter this address in their browser, or scan the code.
            </p>
          </div>

          {info.qr_code_base64_png && (
            <div className="es-server-panel__qr">
              <img src={`data:image/png;base64,${info.qr_code_base64_png}`} alt="Scan to join the examination" width={148} height={148} />
            </div>
          )}
        </div>
      </div>
    </Card>
  )
}
