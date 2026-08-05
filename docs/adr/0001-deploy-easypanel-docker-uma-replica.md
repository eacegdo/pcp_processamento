# Deploy no EasyPanel via Docker com uma réplica

O serviço é um único processo (HTTP + worker FIFO). Decidimos empacotá-lo com Dockerfile no repo, buildar a partir do Git no EasyPanel, expor HTTPS público e rodar **sempre uma réplica**, porque duas instâncias competiriam por Jobs `queued` e quebrariam a regra de um Job por vez. Segredos ficam só em variáveis de ambiente do EasyPanel, nunca na imagem.
