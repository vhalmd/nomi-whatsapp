import './App.css'
import QRCode from 'react-qr-code'
import {QueryClient, QueryClientProvider, useQuery} from "@tanstack/react-query";

const queryClient = new QueryClient()

function App() {
    return (
        <QueryClientProvider client={queryClient}>
            <Home />
        </QueryClientProvider>
    )
}

function Home() {
    const { data, isLoading } = useQuery({
        queryKey: ["qr"],
        queryFn: async () => {
            const raw = await fetch("http://localhost:5555/connect", { method: "POST" })
            const data: {qr: string; status: string} = await raw.json()
            console.log(new Date())

            return data
        },
        refetchInterval: 1000
    })

    return (
        <>
            <div>
                <h2>Scan the QR code below with your Nomi's WhatsappAccount</h2>
            </div>
            {!isLoading && data
                ? <QRCode value={data.qr}/>
                // : <span>NO QR Code available</span>
                : <QRCode value={""}/>
            }
        </>
    )
}

export default App
