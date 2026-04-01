上传：
网站:https://emos.best
上传相关
需 带入 authorization 头部
上传分为 视频 字幕 图片 等, 分为3步 获取基本信息 -> 获取上传token -> 保存上传结果

获取上传token
POST /api/upload/getUploadToken

请求参数
{
    // 资源类型 video 视频 subtitle 字幕 image 图片
    "type": "video",
    // 文件格式
    "file_type": "video/mp4",
    // 文件名称
    "file_name": "demo.mp4",
    // 文件大小 字节
    "file_size": 250817,
    // 储存位置 global 国际 internal 国内 default 默认
    "file_storage": "default"
}
响应内容
{
    // 上传的存储设备 onedrive 
    "type": "onedrive",
    // 文件id
    "file_id": "xWDKXEMv2E",
    // 上传具体参数
    "data": {
        "upload_url": "url"
    }
}
onedrive
let file = File,
    start = 0,
    end = file.size - 1,
    total = file.size

fetch(upload_url, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/octet-stream',
      'Content-Range': `bytes ${start}-${end}/${total}`
    },
    body: file,
})

获取视频目录树：
GET /api/video/tree?type&title&tmdb_id=[tmdb_id]
tmdbid需要手动填写，也可以自己用tmdb api获取
响应：
    {
        "video_type": "tv",
        "item_type": "vl",
        "item_id": 2703,
        "tmdb_id": 1100,
        "todb_id": 2703,
        "title": "老爸老妈的浪漫史",
        "date_air": "2005-09-19",
        "has_media": true,
        "seasons": [
            {
                "item_type": "vs",
                "item_id": 4757,
                "season_title": "特别篇",
                "season_number": 0,
                "date_air": "2006-11-20",
                "has_media": false,
                "episodes": [
                    {
                        "item_type": "ve",
                        "item_id": 80756,
                        "episode_title": "第 1 集",
                        "episode_number": 1,
                        "date_air": "2006-11-20",
                        "has_media": false
                    }, 
                    {
                        "item_type": "ve",
                        "item_id": 80757,
                        "episode_title": "第 2 集",
                        "episode_number": 2,
                        "date_air": "2008-04-21",
                        "has_media": false}
                    },]
                ......]
    }
                {
                "item_type": "vs",
                "item_id": 4766,
                "season_title": "第 9 季",
                "season_number": 9,
                "date_air": "2013-09-23",
                "has_media": true,
                "episodes": [
                    {
                        "item_type": "ve",
                        "item_id": 80950,
                        "episode_title": "吊坠",
                        "episode_number": 1,
                        "date_air": "2013-09-23",
                        "has_media": true
                    },
                    {
                        "item_type": "ve",
                        "item_id": 80951,
                        "episode_title": "会回来的",
                        "episode_number": 2,
                        "date_air": "2013-09-23",
                        "has_media": true
                    },
                    {
                        "item_type": "ve",
                        "item_id": 80952,
                        "episode_title": "在纽约的最后一次",
                        "episode_number": 3,
                        "date_air": "2013-09-30",
                        "has_media": true
                    },
                    ...]}

视频相关
获取基本信息
GET /api/upload/video/base?item_type=[item_type]&item_id=[item_id]

请求参数
item_type 资源类型 电影类型的 vl 或 电视剧类型的 ve
item_id 资源id
响应内容
电影 vl
{
    "title": "阿凡达：水之道"
}
电视集 ve
{
    "video_list_name": "【我推的孩子】",
    "season_number": "S01",
    "episode_number": "E03",
    "episode_title": "漫画原作的电视剧",
    "title": "【我推的孩子】 - S01E03 - 漫画原作的电视剧"
}
保存上传结果
POST /api/upload/video/save

请求参数
{
    "item_type": "vl",
    "item_id": 1,
    "file_id": "xWDKXEMv2E"
}
item_type 和 item_id 与获取基本信息相同
file_id 从获取上传token获取到的
响应内容
正常
{
    // 总上传的数量
    "count": 2,
    // 获得的胡萝卜数量
    "carrot": 0,
    // 媒体资源ID 删除或获取详情时用
    "media_id": "Y1qy9kW9v86K"
}
错误
http code 422

{
    "message": "视频正在合并中 请等待1分钟后再保存试下"
}